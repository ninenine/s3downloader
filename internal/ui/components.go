package ui

import (
	"fyne.io/fyne/v2/widget"
)

// Components struct holds all the UI components for the application
type Components struct {
	BucketEntry       *widget.Entry
	PrefixEntry       *widget.Entry
	FilePathEntry     *widget.Entry
	AwsAccessKeyEntry *widget.Entry
	AwsSecretKeyEntry *widget.Entry
	AwsRegionEntry    *widget.Entry
	ShowSecretCheck   *widget.Check
	OverwriteCheck    *widget.Check
	DownloadButton    *widget.Button
	StopButton        *widget.Button
	StatusLabel       *widget.Label
	ProgressBar       *widget.ProgressBar
}

// NewComponents initializes all the UI components
func NewComponents() *Components {
	c := &Components{
		BucketEntry:       widget.NewEntry(),
		PrefixEntry:       widget.NewEntry(),
		FilePathEntry:     widget.NewEntry(),
		AwsAccessKeyEntry: widget.NewEntry(),
		AwsSecretKeyEntry: widget.NewPasswordEntry(),
		AwsRegionEntry:    widget.NewEntry(),
		ShowSecretCheck:   widget.NewCheck("Show Secret Key", nil),
		OverwriteCheck:    widget.NewCheck("Overwrite existing files", nil),
		DownloadButton:    widget.NewButton("Download", nil),
		StopButton:        widget.NewButton("Stop", nil),
		StatusLabel:       widget.NewLabel("Ready to download"),
		ProgressBar:       widget.NewProgressBar(),
	}

	c.BucketEntry.SetPlaceHolder("Bucket Name")
	c.PrefixEntry.SetPlaceHolder("Prefix (optional)")
	c.FilePathEntry.SetPlaceHolder("Download Path")
	c.AwsAccessKeyEntry.SetPlaceHolder("AWS Access Key (optional)")
	c.AwsSecretKeyEntry.SetPlaceHolder("AWS Secret Key (optional)")
	c.AwsRegionEntry.Text = "eu-west-1"
	c.ProgressBar.Hide()
	c.StopButton.Hide()

	return c
}
