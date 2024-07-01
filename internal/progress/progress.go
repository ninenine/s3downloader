package progress

// Progress struct to track the progress of download operations
type Progress struct {
	FilesFound      int64
	FilesDownloaded int64
	FilesSkipped    int64
}
