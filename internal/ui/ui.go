package ui

import (
	"context"
	"fmt"

	"s3downloader/internal/aws"
	"s3downloader/internal/progress"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// UIManager struct handles the UI lifecycle and interactions
type UIManager struct {
	window     fyne.Window
	downloader *aws.Downloader
	components *Components
	cancelFunc context.CancelFunc
}

// NewUIManager initializes a new UIManager
func NewUIManager(window fyne.Window) *UIManager {
	return &UIManager{
		window:     window,
		components: NewComponents(),
	}
}

// SetupUI sets up the UI components and layout
func (u *UIManager) SetupUI() {
	u.components.DownloadButton.OnTapped = u.StartDownload
	u.components.StopButton.OnTapped = u.StopDownload
	u.components.ShowSecretCheck.OnChanged = func(checked bool) {
		u.components.AwsSecretKeyEntry.Password = !checked
		u.components.AwsSecretKeyEntry.Refresh()
	}

	content := container.NewVBox(
		widget.NewLabel("S3 Downloader"),
		widget.NewForm(
			widget.NewFormItem("Bucket Name", u.components.BucketEntry),
			widget.NewFormItem("Prefix", u.components.PrefixEntry),
			widget.NewFormItem("Download Path", u.components.FilePathEntry),
			widget.NewFormItem("", u.components.OverwriteCheck),
			widget.NewFormItem("AWS Access Key", u.components.AwsAccessKeyEntry),
			widget.NewFormItem("AWS Secret Key", container.NewBorder(nil, nil, nil, u.components.ShowSecretCheck, u.components.AwsSecretKeyEntry)),
			widget.NewFormItem("AWS Region", u.components.AwsRegionEntry),
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewCenter(container.NewHBox(u.components.DownloadButton, u.components.StopButton)),
			widget.NewSeparator(),
		),
		u.components.ProgressBar,
		u.components.StatusLabel,
	)

	paddedContent := container.NewVBox(content)
	u.window.SetContent(container.NewScroll(paddedContent))
}

// StartDownload triggers the download process
func (u *UIManager) StartDownload() {
	bucket := u.components.BucketEntry.Text
	prefix := u.components.PrefixEntry.Text
	downloadPath := u.components.FilePathEntry.Text

	// Validate required fields
	if bucket == "" || downloadPath == "" {
		dialog.ShowInformation("Missing Information", "Please fill in all required fields", u.window)
		return
	}

	u.components.ProgressBar.Show()
	u.disableInputs()

	// Initialize the downloader with AWS credentials
	var err error
	u.downloader, err = aws.NewDownloader(u.components.AwsRegionEntry.Text, u.components.AwsAccessKeyEntry.Text, u.components.AwsSecretKeyEntry.Text)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to create downloader: %w", err), u.window)
		u.enableInputs()
		return
	}

	progressChan := make(chan progress.Progress, 1)
	doneChan := make(chan struct{})

	// Start downloading files
	go u.downloadFiles(bucket, prefix, downloadPath, progressChan, doneChan)
}

func (u *UIManager) downloadFiles(bucket, prefix, downloadPath string, progressChan chan progress.Progress, doneChan chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	u.cancelFunc = cancel

	// Update progress in a separate goroutine
	go func() {
		for p := range progressChan {
			u.updateProgress(p)
		}
		doneChan <- struct{}{}
	}()

	// List and download objects using the downloader
	err := u.downloader.ListAndDownloadObjects(ctx, bucket, prefix, downloadPath, progressChan)

	close(progressChan)
	<-doneChan // Wait for the progress update goroutine to finish

	u.components.ProgressBar.SetValue(0)
	u.components.ProgressBar.Hide()
	u.enableInputs()

	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to list or download objects: %w", err), u.window)
		u.components.StatusLabel.SetText("Failed")
	} else {
		u.components.StatusLabel.SetText("Download complete")
	}
}

// StopDownload cancels the ongoing download process
func (u *UIManager) StopDownload() {
	if u.cancelFunc != nil {
		u.cancelFunc()
	}
}

// updateProgress updates the progress bar and status label
func (u *UIManager) updateProgress(p progress.Progress) {
	filesFound := p.FilesFound
	filesDownloaded := p.FilesDownloaded
	if filesFound == 0 {
		u.components.ProgressBar.SetValue(0)
	} else {
		u.components.ProgressBar.SetValue(float64(filesDownloaded) / float64(filesFound))
	}
	u.components.StatusLabel.SetText(fmt.Sprintf("Files found: %d, Downloaded: %d, Skipped: %d", filesFound, filesDownloaded, p.FilesSkipped))
	u.window.Canvas().Refresh(u.components.ProgressBar)
	fyne.CurrentApp().Driver().CanvasForObject(u.components.StatusLabel).Refresh(u.components.StatusLabel)
}

// disableInputs disables all input fields during the download process
func (u *UIManager) disableInputs() {
	for _, w := range []fyne.Disableable{
		u.components.BucketEntry, u.components.PrefixEntry, u.components.FilePathEntry,
		u.components.AwsAccessKeyEntry, u.components.AwsSecretKeyEntry, u.components.AwsRegionEntry,
		u.components.OverwriteCheck, u.components.DownloadButton, u.components.ShowSecretCheck,
	} {
		w.Disable()
	}
	u.components.StopButton.Show()
}

// enableInputs enables all input fields after the download process
func (u *UIManager) enableInputs() {
	for _, w := range []fyne.Disableable{
		u.components.BucketEntry, u.components.PrefixEntry, u.components.FilePathEntry,
		u.components.AwsAccessKeyEntry, u.components.AwsSecretKeyEntry, u.components.AwsRegionEntry,
		u.components.OverwriteCheck, u.components.DownloadButton, u.components.ShowSecretCheck,
	} {
		w.Enable()
	}
	u.components.StopButton.Hide()
}
