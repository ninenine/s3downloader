package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

// Components struct holds all the UI components for the application
type Components struct {
	// Input fields
	BucketEntry       *widget.Entry
	PrefixEntry       *widget.Entry
	FilePathEntry     *widget.Entry
	AwsAccessKeyEntry *widget.Entry
	AwsSecretKeyEntry *widget.Entry
	AwsRegionEntry    *widget.Entry
	
	// Checkboxes
	ShowSecretCheck   *widget.Check
	OverwriteCheck    *widget.Check
	
	// Buttons
	DownloadButton    *widget.Button
	StopButton        *widget.Button
	BrowseButton      *widget.Button
	BucketValidateBtn *widget.Button
	
	// Feedback and progress components
	StatusLabel       *widget.Label
	ProgressBar       *widget.ProgressBar
	SpeedLabel        *widget.Label
	FileCountLabel    *widget.Label
	BytesLabel        *widget.Label
	
	// Error display
	ErrorsLabel       *widget.Label
	
	// Validation indicators
	BucketValid       *canvas.Rectangle
	PathValid         *canvas.Rectangle
	RegionValid       *canvas.Rectangle
	
	// Main container for the UI
	MainContainer     *fyne.Container
	
	// Reference to the window (set later)
	window            fyne.Window
}

// NewComponents initializes all the UI components
func NewComponents() *Components {
	c := &Components{
		// Initialize input fields
		BucketEntry:       widget.NewEntry(),
		PrefixEntry:       widget.NewEntry(),
		FilePathEntry:     widget.NewEntry(),
		AwsAccessKeyEntry: widget.NewEntry(),
		AwsSecretKeyEntry: widget.NewPasswordEntry(),
		AwsRegionEntry:    widget.NewEntry(),
		
		// Initialize checkboxes with smaller labels
		ShowSecretCheck:   widget.NewCheck("Show", nil),
		OverwriteCheck:    widget.NewCheck("Overwrite existing files", nil),
		
		// Initialize buttons with icons
		DownloadButton:    widget.NewButtonWithIcon("Download", theme.DownloadIcon(), nil),
		StopButton:        widget.NewButtonWithIcon("Stop", theme.CancelIcon(), nil),
		BrowseButton:      widget.NewButtonWithIcon("", theme.FolderOpenIcon(), nil),
		BucketValidateBtn: widget.NewButtonWithIcon("Verify", theme.ConfirmIcon(), nil),
		
		// Initialize status and progress with initial empty text
		StatusLabel:       widget.NewLabel("Ready to download"),
		ProgressBar:       widget.NewProgressBar(),
		SpeedLabel:        widget.NewLabel(""),
		FileCountLabel:    widget.NewLabel(""),
		BytesLabel:        widget.NewLabel(""),
		
		// Initialize error display
		ErrorsLabel:       widget.NewLabel(""),
		
		// Validation indicators (thin colored bars)
		BucketValid:       canvas.NewRectangle(color.Transparent),
		PathValid:         canvas.NewRectangle(color.Transparent),
		RegionValid:       canvas.NewRectangle(color.Transparent),
	}

	// Set up placeholder text and initial values
	c.BucketEntry.SetPlaceHolder("Bucket Name")
	c.PrefixEntry.SetPlaceHolder("Prefix (optional)")
	c.FilePathEntry.SetPlaceHolder("Download Path")
	c.AwsAccessKeyEntry.SetPlaceHolder("AWS Access Key (optional)")
	c.AwsSecretKeyEntry.SetPlaceHolder("AWS Secret Key (optional)")
	c.AwsRegionEntry.Text = "eu-west-1"

	// Configure validation
	c.BucketEntry.Validator = validation.NewRegexp(`^[a-z0-9.-]{3,63}$`, "Invalid bucket name format")
	c.AwsRegionEntry.Validator = validation.NewRegexp(`^[a-z]{2}-[a-z]+-\d$`, "Invalid AWS region format")
	
	// Style elements
	c.ProgressBar.Hide()
	c.StopButton.Hide()
	c.ErrorsLabel.Hide()
	
	// Make validation indicators thin vertical bars
	c.BucketValid.SetMinSize(fyne.NewSize(3, 25))
	c.PathValid.SetMinSize(fyne.NewSize(3, 25))
	c.RegionValid.SetMinSize(fyne.NewSize(3, 25))
	
	// Set up validators that check input as user types
	c.BucketEntry.OnChanged = c.updateBucketValidation
	c.FilePathEntry.OnChanged = c.updatePathValidation
	c.AwsRegionEntry.OnChanged = c.updateRegionValidation
	
	// Initial validation
	c.updateBucketValidation(c.BucketEntry.Text)
	c.updatePathValidation(c.FilePathEntry.Text)
	c.updateRegionValidation(c.AwsRegionEntry.Text)
	
	// Create the main layout
	c.createMainContainer()

	return c
}

// SetWindow configures window-dependent components like file pickers
func (c *Components) SetWindow(win fyne.Window) {
	c.window = win
	
	// Setup file picker dialog for the download path using native file dialog
	c.BrowseButton.OnTapped = func() {
		fd := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			c.FilePathEntry.SetText(uri.Path())
			c.updatePathValidation(c.FilePathEntry.Text)
		}, win)
		
		// Use system dialog if available
		fd.SetConfirmText("Select")
		fd.SetDismissText("Cancel")
		fd.Resize(fyne.NewSize(700, 500))
		
		// Try to use the native dialog
		if drv, ok := fyne.CurrentApp().Driver().(interface{ FileDialog() bool }); ok {
			if drv.FileDialog() {
				fd.Show()
				return
			}
		}
		
		// Fall back to Fyne dialog if native dialog not available
		fd.Show()
	}
}

// updateBucketValidation validates the bucket name format
func (c *Components) updateBucketValidation(text string) {
	if c.BucketEntry.Validate() == nil && text != "" {
		c.BucketValid.FillColor = color.NRGBA{R: 0, G: 180, B: 0, A: 255}
	} else {
		c.BucketValid.FillColor = color.NRGBA{R: 180, G: 0, B: 0, A: 255}
	}
	c.BucketValid.Refresh()
}

// updatePathValidation checks if the path is valid
func (c *Components) updatePathValidation(text string) {
	if text != "" {
		c.PathValid.FillColor = color.NRGBA{R: 0, G: 180, B: 0, A: 255}
	} else {
		c.PathValid.FillColor = color.NRGBA{R: 180, G: 0, B: 0, A: 255}
	}
	c.PathValid.Refresh()
}

// updateRegionValidation validates the AWS region format
func (c *Components) updateRegionValidation(text string) {
	if c.AwsRegionEntry.Validate() == nil {
		c.RegionValid.FillColor = color.NRGBA{R: 0, G: 180, B: 0, A: 255}
	} else {
		c.RegionValid.FillColor = color.NRGBA{R: 180, G: 0, B: 0, A: 255}
	}
	c.RegionValid.Refresh()
}

// createMainContainer sets up the entire UI layout
func (c *Components) createMainContainer() {
	// Source section (S3 bucket info)
	bucketRow := container.NewBorder(nil, nil, nil, container.NewHBox(c.BucketValidateBtn, c.BucketValid), c.BucketEntry)
	sourceForm := widget.NewForm(
		widget.NewFormItem("Bucket:", bucketRow),
		widget.NewFormItem("Prefix:", c.PrefixEntry),
	)
	
	// Destination section (local path)
	pathRow := container.NewBorder(nil, nil, nil, container.NewHBox(c.BrowseButton, c.PathValid), c.FilePathEntry)
	destForm := widget.NewForm(
		widget.NewFormItem("Download Path:", pathRow),
		widget.NewFormItem("Options:", c.OverwriteCheck),
	)
	
	// AWS credentials section
	awsForm := widget.NewForm(
		widget.NewFormItem("Access Key:", c.AwsAccessKeyEntry),
		widget.NewFormItem("Secret Key:", container.NewBorder(nil, nil, nil, c.ShowSecretCheck, c.AwsSecretKeyEntry)),
		widget.NewFormItem("Region:", container.NewBorder(nil, nil, nil, c.RegionValid, c.AwsRegionEntry)),
	)
	
	// Input section with tabs (combines source, destination, and AWS)
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Source", theme.StorageIcon(), sourceForm),
		container.NewTabItemWithIcon("Destination", theme.FolderIcon(), destForm),
		container.NewTabItemWithIcon("AWS", theme.AccountIcon(), awsForm),
	)
	
	// Make tabs take minimal space
	tabs.SetTabLocation(container.TabLocationTop)
	
	// Set initial values for progress labels
	if c.FileCountLabel.Text == "" {
		c.FileCountLabel.SetText("Files: 0 / 0 (0 skipped)")
	}
	if c.BytesLabel.Text == "" {
		c.BytesLabel.SetText("Size: 0 B")
	}
	if c.SpeedLabel.Text == "" {
		c.SpeedLabel.SetText("Speed: - B/s")
	}
	
	// Progress tracking section
	progressInfo := container.NewHBox(
		container.NewHBox(widget.NewIcon(theme.DocumentIcon()), c.FileCountLabel),
		container.NewHBox(widget.NewIcon(theme.StorageIcon()), c.BytesLabel),
		container.NewHBox(widget.NewIcon(theme.UploadIcon()), c.SpeedLabel),
	)
	
	progressSection := container.NewVBox(
		c.ProgressBar,
		container.NewCenter(progressInfo),
		c.StatusLabel,
		c.ErrorsLabel,
	)
	
	// Button section
	buttonSection := container.NewCenter(
		container.NewHBox(
			c.DownloadButton,
			c.StopButton,
		),
	)
	
	// Combine everything in a BorderLayout
	c.MainContainer = container.NewBorder(
		tabs, // Top
		container.NewVBox(progressSection, buttonSection), // Bottom
		nil,  // Left
		nil,  // Right
		nil,  // Center - empty since we're using top and bottom
	)
}

// GetMainContainer returns the main UI container
func (c *Components) GetMainContainer() fyne.CanvasObject {
	return c.MainContainer
}