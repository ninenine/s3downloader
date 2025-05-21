package main

import (
	"log"
	"os"
	"runtime/debug"
	"s3downloader/internal/ui"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
)

// Build info for version tracking
var (
	version = "dev"
	commit  = "unknown"
)

// resourceIconPng reads and returns the content of the icon.png file
func resourceIconPng() []byte {
	data, err := os.ReadFile("icon.png")
	if err != nil {
		// Return a simple empty image if icon not found
		return []byte{}
	}
	return data
}

func main() {
	// Setup error recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("APPLICATION PANIC: %v\n%s", r, debug.Stack())
			// If we're in a panic state, attempt to create a log file
			file, err := os.Create("s3downloader-crash.log")
			if err == nil {
				defer file.Close()
				file.WriteString("S3 Downloader Crash Report\n\n")
				file.WriteString("Error: ")
				file.WriteString(r.(string))
				file.WriteString("\n\nStack Trace:\n")
				file.Write(debug.Stack())
			}
			os.Exit(1)
		}
	}()

	// Initialize the application with an ID and preferences
	myApp := app.NewWithID("com.ninenine.s3downloader")
	myApp.SetIcon(fyne.NewStaticResource("icon", resourceIconPng()))

	// Create a new window for the application with a more descriptive title
	appTitle := "S3 Downloader"
	if version != "dev" {
		appTitle += " v" + version
	}
	
	// Create and set up the main window
	myWindow := myApp.NewWindow(appTitle)
	myWindow.Resize(fyne.NewSize(600, 500))
	myWindow.SetMaster()
	myWindow.SetFixedSize(false) // Allow resizing
	myWindow.CenterOnScreen()
	
	// Setup window close handler to confirm exit using native dialog if possible
	myWindow.SetCloseIntercept(func() {
		confirmDialog := dialog.NewConfirm(
			"Exit Application", 
			"Are you sure you want to exit? Any ongoing downloads will be canceled.", 
			func(ok bool) {
				if ok {
					myWindow.Close()
				}
			}, 
			myWindow,
		)
		confirmDialog.SetDismissText("Cancel")
		confirmDialog.SetConfirmText("Exit")
		confirmDialog.Show()
	})

	// Initialize the UI manager and setup the UI
	uiManager := ui.NewUIManager(myWindow)
	uiManager.SetupUI()

	// Show and run the window
	myWindow.ShowAndRun()
}
