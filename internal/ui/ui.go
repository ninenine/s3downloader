package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"s3downloader/internal/aws"
	"s3downloader/internal/progress"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
)

const (
	// Used for download speed calculation
	updateInterval = 500 * time.Millisecond
	
	// Default buffer size for progress channel
	progressBufferSize = 100
)

// DownloadState tracks the current download session
type DownloadState struct {
	startTime    time.Time
	lastBytes    int64
	lastUpdate   time.Time
	bytesPerSec  float64
	cancelFunc   context.CancelFunc
	progressChan chan progress.Progress
}

// UIManager struct handles the UI lifecycle and interactions
type UIManager struct {
	window            fyne.Window
	downloader        *aws.Downloader
	components        *Components
	state             *DownloadState
}

// NewUIManager initializes a new UIManager
func NewUIManager(window fyne.Window) *UIManager {
	manager := &UIManager{
		window:     window,
		components: NewComponents(),
		state:      &DownloadState{},
	}
	
	// Add window reference after components are created
	manager.components.SetWindow(window)
	
	return manager
}

// SetupUI sets up the UI components and layout
func (u *UIManager) SetupUI() {
	// Set up event handlers
	u.components.DownloadButton.OnTapped = u.StartDownload
	u.components.StopButton.OnTapped = u.StopDownload
	u.components.BucketValidateBtn.OnTapped = u.ValidateBucket
	
	u.components.ShowSecretCheck.OnChanged = func(checked bool) {
		u.components.AwsSecretKeyEntry.Password = !checked
		u.components.AwsSecretKeyEntry.Refresh()
	}

	// Set the window content with padding and scrolling capability
	paddedContent := container.NewPadded(u.components.GetMainContainer())
	u.window.SetContent(container.NewScroll(paddedContent))
}

// ValidateBucket verifies if the bucket exists and is accessible
func (u *UIManager) ValidateBucket() {
	bucket := u.components.BucketEntry.Text
	if bucket == "" {
		d := dialog.NewError(fmt.Errorf("bucket name cannot be empty"), u.window)
		d.Show()
		return
	}

	// Temporarily disable the button to prevent multiple clicks
	u.components.BucketValidateBtn.Disable()
	u.components.BucketValidateBtn.SetText("Checking...")
	
	// Create AWS downloader for validation
	region := u.components.AwsRegionEntry.Text
	accessKey := u.components.AwsAccessKeyEntry.Text
	secretKey := u.components.AwsSecretKeyEntry.Text
	
	// Run validation in a goroutine to keep UI responsive
	go func() {
		defer func() {
			u.components.BucketValidateBtn.Enable()
			u.components.BucketValidateBtn.SetText("Verify")
		}()
		
		var err error
		u.downloader, err = aws.NewDownloader(region, accessKey, secretKey)
		if err != nil {
			dlg := dialog.NewError(fmt.Errorf("AWS connection error: %w", err), u.window)
			dlg.Show()
			return
		}
		
		err = u.downloader.ValidateBucketExists(bucket)
		if err != nil {
			dlg := dialog.NewError(fmt.Errorf("bucket validation failed: %w", err), u.window)
			dlg.Show()
			return
		}
		
		// Success with native dialog
		infoDialog := dialog.NewInformation("Success", 
			fmt.Sprintf("Connected to bucket '%s'", bucket), 
			u.window)
		infoDialog.Show()
	}()
}

// StartDownload triggers the download process
func (u *UIManager) StartDownload() {
	// Get input values
	bucket := u.components.BucketEntry.Text
	prefix := u.components.PrefixEntry.Text
	downloadPath := u.components.FilePathEntry.Text
	overwrite := u.components.OverwriteCheck.Checked

	// Validate required fields
	if bucket == "" || downloadPath == "" {
		dlg := dialog.NewInformation("Missing Information", "Please fill in all required fields", u.window)
		dlg.Show()
		return
	}
	
	// Validate inputs
	if u.components.BucketEntry.Validate() != nil {
		dlg := dialog.NewError(fmt.Errorf("invalid bucket name format"), u.window)
		dlg.Show()
		return
	}
	
	if u.components.AwsRegionEntry.Validate() != nil {
		dlg := dialog.NewError(fmt.Errorf("invalid AWS region format"), u.window)
		dlg.Show()
		return
	}
	
	// Prepare UI for download
	u.prepareUIForDownload()

	// Initialize AWS downloader
	region := u.components.AwsRegionEntry.Text
	accessKey := u.components.AwsAccessKeyEntry.Text
	secretKey := u.components.AwsSecretKeyEntry.Text
	
	var err error
	u.downloader, err = aws.NewDownloader(region, accessKey, secretKey)
	if err != nil {
		u.handleDownloadError(fmt.Errorf("failed to create AWS downloader: %w", err))
		return
	}

	// Create download state with channels and context
	ctx, cancel := context.WithCancel(context.Background())
	u.state.cancelFunc = cancel
	u.state.progressChan = make(chan progress.Progress, progressBufferSize)
	u.state.startTime = time.Now()
	u.state.lastUpdate = time.Now()
	
	// Set initial UI status - keep text minimal to avoid overflow
	u.components.StatusLabel.SetText("Starting...")
	u.components.FileCountLabel.SetText("Files: 0/0")
	u.components.FileCountLabel.Show()
	u.components.BytesLabel.SetText("Size: 0 B")
	u.components.BytesLabel.Show()
	u.components.SpeedLabel.SetText("0 B/s")
	u.components.SpeedLabel.Show()
	
	// Start progress updater in a separate goroutine
	go u.progressUpdater()
	
	// Start downloading files in a background goroutine
	go u.downloadFiles(ctx, bucket, prefix, downloadPath, overwrite)
}

// downloadFiles performs the actual download operation
func (u *UIManager) downloadFiles(ctx context.Context, bucket, prefix, downloadPath string, overwrite bool) {
	// Normalize and prepare the download path
	downloadPath = filepath.Clean(downloadPath)
	
	// Start download operation
	err := u.downloader.ListAndDownloadObjects(
		ctx, 
		bucket, 
		prefix, 
		downloadPath, 
		overwrite, 
		u.state.progressChan,
	)
	
	// Close progress channel when done
	close(u.state.progressChan)

	// Handle the result
	if err != nil {
		if errors.Is(err, aws.ErrDownloadCanceled) {
			// User cancelled the download - simple text
			u.components.StatusLabel.SetText("Canceled")
			u.cleanupAfterDownload()
		} else {
			// Some other error occurred - use native dialog if possible
			dlg := dialog.NewError(err, u.window)
			dlg.Show()
			u.components.StatusLabel.SetText("Error")
			u.cleanupAfterDownload()
		}
	} else {
		// Download completed successfully - short message
		u.components.StatusLabel.SetText("Complete!")
		u.cleanupAfterDownload()
	}
}

// progressUpdater monitors the progress channel and updates the UI
func (u *UIManager) progressUpdater() {
	var lastDisplayTime time.Time
	
	for p := range u.state.progressChan {
		now := time.Now()
		
		// Limit UI updates to a reasonable rate to prevent flickering
		if now.Sub(lastDisplayTime) >= updateInterval {
			lastDisplayTime = now
			
			// Update UI safely
			u.updateProgressUI(p)
		}
	}
}

// updateProgressUI updates the UI with progress information
func (u *UIManager) updateProgressUI(p progress.Progress) {
	// Update progress bar
	if p.FilesFound > 0 {
		u.components.ProgressBar.SetValue(float64(p.FilesDownloaded) / float64(p.FilesFound))
	} else {
		u.components.ProgressBar.SetValue(0)
	}

	// Calculate and update download speed
	now := time.Now()
	elapsed := now.Sub(u.state.lastUpdate).Seconds()
	if elapsed > 0 {
		bytesDiff := p.TotalBytes - u.state.lastBytes
		if bytesDiff >= 0 {
			u.state.bytesPerSec = float64(bytesDiff) / elapsed
			u.state.lastBytes = p.TotalBytes
			u.state.lastUpdate = now
		}
	}
	
	// Format and display minimal stats to avoid buffer overflow
	u.components.FileCountLabel.SetText(fmt.Sprintf("Files: %d/%d", 
		p.FilesDownloaded, p.FilesFound))
	
	// Show size
	u.components.BytesLabel.SetText(fmt.Sprintf("Size: %s", formatBytes(p.TotalBytes)))
	
	// Show speed 
	u.components.SpeedLabel.SetText(fmt.Sprintf("%s/s", formatBytes(int64(u.state.bytesPerSec))))
	
	// Show skipped files count in status
	if p.FilesSkipped > 0 {
		u.components.StatusLabel.SetText(fmt.Sprintf("Skipped: %d", p.FilesSkipped))
	} else {
		u.components.StatusLabel.SetText("Downloading...")
	}
	
	// Show errors count if any
	if p.ErrorCount > 0 {
		u.components.ErrorsLabel.SetText(fmt.Sprintf("Errors: %d", p.ErrorCount))
		u.components.ErrorsLabel.Show()
	} else {
		u.components.ErrorsLabel.Hide()
	}
}

// StopDownload cancels the ongoing download process
func (u *UIManager) StopDownload() {
	if u.state.cancelFunc != nil {
		u.state.cancelFunc()
		u.components.StatusLabel.SetText("Stopping...")
		u.components.StopButton.Disable()
	}
}

// handleDownloadError handles download errors and updates the UI
func (u *UIManager) handleDownloadError(err error) {
	dlg := dialog.NewError(err, u.window)
	dlg.Show()
	u.cleanupAfterDownload()
}

// prepareUIForDownload prepares the UI for download operation
func (u *UIManager) prepareUIForDownload() {
	u.components.ProgressBar.Show()
	u.components.StatusLabel.SetText("Preparing...")
	u.disableInputs()
}

// cleanupAfterDownload resets the UI after download completion
func (u *UIManager) cleanupAfterDownload() {
	u.enableInputs()
	u.state.cancelFunc = nil
}

// disableInputs disables all input fields during the download process
func (u *UIManager) disableInputs() {
	// Disable input fields
	u.components.BucketEntry.Disable()
	u.components.PrefixEntry.Disable()
	u.components.FilePathEntry.Disable()
	u.components.AwsAccessKeyEntry.Disable()
	u.components.AwsSecretKeyEntry.Disable()
	u.components.AwsRegionEntry.Disable()
	
	// Disable checkboxes and buttons
	u.components.OverwriteCheck.Disable()
	u.components.ShowSecretCheck.Disable()
	u.components.DownloadButton.Disable()
	u.components.BrowseButton.Disable()
	u.components.BucketValidateBtn.Disable()
	
	// Show the stop button
	u.components.StopButton.Show()
	u.components.StopButton.Enable()
}

// enableInputs enables all input fields after the download process
func (u *UIManager) enableInputs() {
	// Enable input fields
	u.components.BucketEntry.Enable()
	u.components.PrefixEntry.Enable()
	u.components.FilePathEntry.Enable()
	u.components.AwsAccessKeyEntry.Enable()
	u.components.AwsSecretKeyEntry.Enable()
	u.components.AwsRegionEntry.Enable()
	
	// Enable checkboxes and buttons
	u.components.OverwriteCheck.Enable()
	u.components.ShowSecretCheck.Enable()
	u.components.DownloadButton.Enable()
	u.components.BrowseButton.Enable()
	u.components.BucketValidateBtn.Enable()
	
	// Hide the stop button
	u.components.StopButton.Hide()
}

// formatElapsedTime formats a duration into a human-readable string
func formatElapsedTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// formatBytes converts bytes to a human-readable format
func formatBytes(bytes int64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
	)

	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	
	if bytes < int64(MB) {
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	}
	
	if bytes < int64(GB) {
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	}
	
	if bytes < int64(TB) {
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	}
	
	return fmt.Sprintf("%.1f TB", float64(bytes)/TB)
}