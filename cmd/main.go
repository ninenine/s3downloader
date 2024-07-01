package main

import (
	"s3downloader/internal/ui"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	myApp := app.NewWithID("com.ninenine.s3downloader")
	myWindow := myApp.NewWindow("S3 Downloader")
	myWindow.Resize(fyne.NewSize(800, 480))

	uiManager := ui.NewUIManager(myWindow)
	uiManager.SetupUI()

	myWindow.ShowAndRun()
}
