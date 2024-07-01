package main

import (
	"s3downloader/internal/ui"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	// Initialize the application with an ID
	myApp := app.NewWithID("com.ninenine.s3downloader")

	// Create a new window for the application
	myWindow := myApp.NewWindow("S3 Downloader")
	myWindow.Resize(fyne.NewSize(800, 480))

	// Initialize the UI manager and setup the UI
	uiManager := ui.NewUIManager(myWindow)
	uiManager.SetupUI()

	// Show and run the window
	myWindow.ShowAndRun()
}
