package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"s3downloader/internal/progress"
	"s3downloader/pkg/fileutils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// ErrDownloadCanceled is returned when download is canceled by context
var ErrDownloadCanceled = errors.New("download operation canceled")

// Config holds downloader configuration options
type Config struct {
	MaxWorkers      int
	PartSize        int64
	Concurrency     int
	DownloadTimeout time.Duration
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() Config {
	return Config{
		MaxWorkers:      runtime.NumCPU() * 4,
		PartSize:        10 * 1024 * 1024, // 10MB chunk size
		Concurrency:     10,
		DownloadTimeout: 30 * time.Minute,
	}
}

// Downloader struct handles AWS sessions and S3 operations
type Downloader struct {
	sess   *session.Session
	s3     *s3.S3
	config Config
}

// NewDownloader initializes a new Downloader with AWS credentials
func NewDownloader(region, accessKey, secretKey string) (*Downloader, error) {
	return NewDownloaderWithConfig(region, accessKey, secretKey, DefaultConfig())
}

// NewDownloaderWithConfig initializes a new Downloader with AWS credentials and custom config
func NewDownloaderWithConfig(region, accessKey, secretKey string, config Config) (*Downloader, error) {
	if region == "" {
		return nil, fmt.Errorf("AWS region cannot be empty")
	}

	awsConfig := &aws.Config{
		Region: aws.String(region),
	}

	if accessKey != "" && secretKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}

	// Add reasonable retry configuration
	awsConfig.MaxRetries = aws.Int(3)

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &Downloader{
		sess:   sess,
		s3:     s3.New(sess),
		config: config,
	}, nil
}

// ListAndDownloadObjects lists and downloads S3 objects concurrently
func (d *Downloader) ListAndDownloadObjects(
	ctx context.Context,
	bucket, prefix, downloadPath string,
	overwrite bool,
	progressChan chan<- progress.Progress,
) error {
	if bucket == "" {
		return fmt.Errorf("S3 bucket name cannot be empty")
	}

	if downloadPath == "" {
		return fmt.Errorf("download path cannot be empty")
	}

	// Validate download path exists
	if !fileutils.FileExists(downloadPath) {
		if err := fileutils.EnsureDirectoryExists(downloadPath); err != nil {
			return fmt.Errorf("download path doesn't exist and couldn't be created: %w", err)
		}
	}

	// Initialize atomic counters for thread-safe operations
	var (
		foundFiles     int64
		processedFiles int64
		skippedFiles   int64
		totalBytes     int64 // Track total bytes downloaded
		errorCount     int64 // Track error count
	)

	// Create buffered channels for communication
	fileChan := make(chan *s3.Object, 1000)
	errChan := make(chan error, d.config.MaxWorkers)
	doneChan := make(chan struct{})
	listingDone := make(chan struct{})
	var wg sync.WaitGroup

	// Create downloader with configured options
	downloader := s3manager.NewDownloader(d.sess, func(dldr *s3manager.Downloader) {
		dldr.PartSize = d.config.PartSize
		dldr.Concurrency = d.config.Concurrency
	})

	// Start worker pool for downloading files
	for i := 0; i < d.config.MaxWorkers; i++ {
		wg.Add(1)
		go d.downloadWorker(
			ctx,
			bucket,
			downloadPath,
			overwrite,
			downloader,
			fileChan,
			errChan,
			&wg,
			&processedFiles,
			&skippedFiles,
			&foundFiles,
			&totalBytes,
			progressChan,
		)
	}

	// List objects and send them to the fileChan
	go func() {
		defer close(listingDone)

		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		}

		err := d.s3.ListObjectsV2PagesWithContext(
			ctx,
			input,
			func(page *s3.ListObjectsV2Output, lastPage bool) bool {
				for _, obj := range page.Contents {
					// Skip directories (objects with trailing slash and 0 size)
					if aws.Int64Value(obj.Size) == 0 && filepath.Base(aws.StringValue(obj.Key)) == "" {
						continue
					}

					select {
					case fileChan <- obj:
						// Update the files found counter and report progress
						atomic.AddInt64(&foundFiles, 1)

						currentValues := progress.Progress{
							FilesFound:      atomic.LoadInt64(&foundFiles),
							FilesDownloaded: atomic.LoadInt64(&processedFiles) - atomic.LoadInt64(&skippedFiles),
							FilesSkipped:    atomic.LoadInt64(&skippedFiles),
							TotalBytes:      atomic.LoadInt64(&totalBytes),
						}

						select {
						case progressChan <- currentValues:
							// Progress sent successfully
						case <-ctx.Done():
							return false
						default:
							// Channel full, continue without blocking
						}

					case <-ctx.Done():
						return false
					}
				}
				return !lastPage
			},
		)

		if err != nil {
			if !errors.Is(err, context.Canceled) {
				select {
				case errChan <- fmt.Errorf("error listing objects: %w", err):
				default:
					// Don't block if channel is full
				}
			}
		}
	}()

	// Wait for all workers to finish and close channels
	go func() {
		// Wait for the listing to finish first
		<-listingDone
		// Now close the file channel to signal workers there's no more work
		close(fileChan)
		
		// Wait for all workers to finish
		wg.Wait()
		
		// Close the error channel
		close(errChan)
		
		// Signal that all workers have finished
		close(doneChan)
	}()

	// Wait for completion or cancellation
	select {
	case <-doneChan:
		// All downloading completed
	case <-ctx.Done():
		// Context canceled
		return ErrDownloadCanceled
	}

	// Collect and handle errors
	var errs []error
	for e := range errChan {
		if e != nil {
			errs = append(errs, e)
			atomic.AddInt64(&errorCount, 1)
		}
	}

	// Final progress update
	finalProgress := progress.Progress{
		FilesFound:      atomic.LoadInt64(&foundFiles),
		FilesDownloaded: atomic.LoadInt64(&processedFiles) - atomic.LoadInt64(&skippedFiles),
		FilesSkipped:    atomic.LoadInt64(&skippedFiles),
		TotalBytes:      atomic.LoadInt64(&totalBytes),
		ErrorCount:      atomic.LoadInt64(&errorCount),
	}

	select {
	case progressChan <- finalProgress:
	default:
		// Don't block if channel is full or closed
	}

	// Return first error if any occurred
	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors during download. First error: %w", len(errs), errs[0])
	}

	return nil
}

// downloadWorker processes each file from the channel
func (d *Downloader) downloadWorker(
	ctx context.Context,
	bucket, downloadPath string,
	overwrite bool,
	downloader *s3manager.Downloader,
	fileChan <-chan *s3.Object,
	errChan chan<- error,
	wg *sync.WaitGroup,
	processedFiles, skippedFiles, foundFiles, totalBytes *int64,
	progressChan chan<- progress.Progress,
) {
	defer wg.Done()

	for file := range fileChan {
		select {
		case <-ctx.Done():
			return
		default:
			key := aws.StringValue(file.Key)
			size := aws.Int64Value(file.Size)
			localFilePath := filepath.Join(downloadPath, key)
			localDir := filepath.Dir(localFilePath)

			if err := fileutils.EnsureDirectoryExists(localDir); err != nil {
				errChan <- fmt.Errorf("failed to create directory for '%s': %w", key, err)
				continue
			}

			// Skip if file exists and user didn't choose "overwrite"
			if fileutils.FileExists(localFilePath) && !overwrite {
				atomic.AddInt64(skippedFiles, 1)
				atomic.AddInt64(processedFiles, 1)
				
				// Send progress update
				currentProgress := progress.Progress{
					FilesFound:      atomic.LoadInt64(foundFiles),
					FilesDownloaded: atomic.LoadInt64(processedFiles) - atomic.LoadInt64(skippedFiles),
					FilesSkipped:    atomic.LoadInt64(skippedFiles),
					TotalBytes:      atomic.LoadInt64(totalBytes),
				}
				
				select {
				case progressChan <- currentProgress:
					// Progress sent successfully
				default:
					// Skip if channel is full to prevent blocking
				}
				
				continue
			}

			// Perform the actual download
			err := d.downloadFile(ctx, downloader, bucket, key, localFilePath)
			if err != nil {
				errChan <- err
				continue
			}

			// Update counters
			atomic.AddInt64(processedFiles, 1)
			atomic.AddInt64(totalBytes, size)
			
			// Send progress update
			currentProgress := progress.Progress{
				FilesFound:      atomic.LoadInt64(foundFiles),
				FilesDownloaded: atomic.LoadInt64(processedFiles) - atomic.LoadInt64(skippedFiles),
				FilesSkipped:    atomic.LoadInt64(skippedFiles),
				TotalBytes:      atomic.LoadInt64(totalBytes),
			}
			
			select {
			case progressChan <- currentProgress:
				// Progress sent successfully
			default:
				// Skip if channel is full to prevent blocking
			}
		}
	}
}

// downloadFile downloads any file (small or large) from S3
func (d *Downloader) downloadFile(
	ctx context.Context,
	downloader *s3manager.Downloader,
	bucket, key, localPath string,
) error {
	// Create parent directories if they don't exist
	if err := fileutils.EnsureDirectoryExists(filepath.Dir(localPath)); err != nil {
		return fmt.Errorf("failed to create parent directory for '%s': %w", key, err)
	}

	// Create local file
	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file '%s': %w", key, err)
	}
	defer f.Close()

	// Use a timeout for the download
	downloadCtx, cancel := context.WithTimeout(ctx, d.config.DownloadTimeout)
	defer cancel()

	// Perform the download with the AWS SDK
	_, err = downloader.DownloadWithContext(
		downloadCtx,
		f,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
	)
	
	if err != nil {
		// Clean up partial file on error
		f.Close() // Ensure file is closed before removal
		os.Remove(localPath)
		
		// Check if it was a cancellation
		if errors.Is(err, context.Canceled) {
			return ErrDownloadCanceled
		}
		
		return fmt.Errorf("failed to download '%s': %w", key, err)
	}
	
	return nil
}

// ListPrefixes lists prefixes (subdirectories) within a given S3 bucket and prefix
func (d *Downloader) ListPrefixes(bucket, prefix string) ([]string, error) {
	if bucket == "" {
		return nil, fmt.Errorf("S3 bucket name cannot be empty")
	}

	var prefixes []string
	err := d.s3.ListObjectsV2Pages(
		&s3.ListObjectsV2Input{
			Bucket:    aws.String(bucket),
			Delimiter: aws.String("/"),
			Prefix:    aws.String(prefix),
		},
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, p := range page.CommonPrefixes {
				prefixes = append(prefixes, aws.StringValue(p.Prefix))
			}
			return !lastPage
		},
	)
	
	if err != nil {
		return nil, fmt.Errorf("error listing prefixes: %w", err)
	}
	
	return prefixes, nil
}

// ValidateBucketExists checks if the bucket exists and is accessible
func (d *Downloader) ValidateBucketExists(bucket string) error {
	if bucket == "" {
		return fmt.Errorf("S3 bucket name cannot be empty")
	}

	_, err := d.s3.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	
	if err != nil {
		return fmt.Errorf("cannot access bucket '%s': %w", bucket, err)
	}
	
	return nil
}