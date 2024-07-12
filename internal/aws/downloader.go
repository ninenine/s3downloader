package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// Downloader struct handles AWS sessions and S3 operations
type Downloader struct {
	sess *session.Session
	s3   *s3.S3
}

// NewDownloader initializes a new Downloader with AWS credentials
func NewDownloader(region, accessKey, secretKey string) (*Downloader, error) {
	config := &aws.Config{
		Region: aws.String(region),
	}
	if accessKey != "" && secretKey != "" {
		config.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}
	sess, err := session.NewSession(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	return &Downloader{sess: sess, s3: s3.New(sess)}, nil
}

// ListAndDownloadObjects lists and downloads S3 objects concurrently
func (d *Downloader) ListAndDownloadObjects(ctx context.Context, bucket, prefix, downloadPath string, progressChan chan<- progress.Progress) error {
	const (
		maxWorkers        = 100              // Maximum number of workers
		chunkSize         = 10 * 1024 * 1024 // 10MB chunks for large files
		channelBufferSize = 2000             // Channel buffer size
	)

	var (
		foundFiles     int64
		processedFiles int64
		skippedFiles   int64
	)

	fileChan := make(chan *s3.Object, channelBufferSize)
	errChan := make(chan error, maxWorkers)
	doneChan := make(chan struct{})
	var wg sync.WaitGroup

	downloader := s3manager.NewDownloader(d.sess, func(d *s3manager.Downloader) {
		d.PartSize = chunkSize
		d.Concurrency = 10
	})

	// Start worker pool based on file size
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go d.downloadWorker(ctx, bucket, downloadPath, downloader, fileChan, errChan, &wg, chunkSize, &processedFiles, &skippedFiles, progressChan)
	}

	// List objects and send to channel
	go func() {
		defer close(fileChan)
		defer close(doneChan)
		err := d.s3.ListObjectsV2PagesWithContext(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				select {
				case fileChan <- obj:
					atomic.AddInt64(&foundFiles, 1)
					progressChan <- progress.Progress{FilesFound: foundFiles, FilesDownloaded: processedFiles, FilesSkipped: skippedFiles}
				case <-ctx.Done():
					return false
				}
			}
			return !lastPage
		})
		if err != nil {
			errChan <- fmt.Errorf("error listing objects: %w", err)
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	select {
	case <-doneChan:
		// Listing completed
	case <-ctx.Done():
		// Context canceled
		return ctx.Err()
	}

	// Check for errors from downloading
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// downloadWorker processes the download of each file
func (d *Downloader) downloadWorker(ctx context.Context, bucket, downloadPath string, downloader *s3manager.Downloader,
	fileChan <-chan *s3.Object, errChan chan<- error, wg *sync.WaitGroup, chunkSize int64,
	processedFiles, skippedFiles *int64, progressChan chan<- progress.Progress) {
	defer wg.Done()

	for file := range fileChan {
		select {
		case <-ctx.Done():
			return
		default:
			localFilePath := filepath.Join(downloadPath, aws.StringValue(file.Key))
			localDir := filepath.Dir(localFilePath)

			// Ensure that the directory exists before attempting to create the file
			if err := fileutils.EnsureDirectoryExists(localDir); err != nil {
				errChan <- fmt.Errorf("failed to create directory for '%s': %w", aws.StringValue(file.Key), err)
				continue
			}

			// Skip files that already exist
			if fileutils.FileExists(localFilePath) {
				atomic.AddInt64(skippedFiles, 1)
				atomic.AddInt64(processedFiles, 1)
				progressChan <- progress.Progress{FilesFound: atomic.LoadInt64(processedFiles), FilesDownloaded: atomic.LoadInt64(processedFiles), FilesSkipped: atomic.LoadInt64(skippedFiles)}
				continue
			}

			// Proceed to download the file
			if aws.Int64Value(file.Size) > chunkSize {
				if err := d.downloadLargeFile(ctx, downloader, bucket, file.Key, localFilePath); err != nil {
					errChan <- err
				} else {
					atomic.AddInt64(processedFiles, 1)
					progressChan <- progress.Progress{FilesFound: atomic.LoadInt64(processedFiles), FilesDownloaded: atomic.LoadInt64(processedFiles), FilesSkipped: atomic.LoadInt64(skippedFiles)}
				}
			} else {
				if err := d.downloadSmallFile(ctx, downloader, bucket, file.Key, localFilePath); err != nil {
					errChan <- err
				} else {
					atomic.AddInt64(processedFiles, 1)
					progressChan <- progress.Progress{FilesFound: atomic.LoadInt64(processedFiles), FilesDownloaded: atomic.LoadInt64(processedFiles), FilesSkipped: atomic.LoadInt64(skippedFiles)}
				}
			}
		}
	}
}

// downloadSmallFile downloads a small file (<chunkSize) from S3
func (d *Downloader) downloadSmallFile(ctx context.Context, downloader *s3manager.Downloader, bucket string, key *string, localPath string) error {
	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file '%s': %w", aws.StringValue(key), err)
	}
	defer f.Close()

	downloadCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	_, err = downloader.DownloadWithContext(downloadCtx, f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    key,
	})

	if err != nil {
		os.Remove(localPath) // Clean up partially downloaded file
		return fmt.Errorf("failed to download '%s': %w", aws.StringValue(key), err)
	}

	return nil
}

// downloadLargeFile downloads a large file (>=chunkSize) from S3 using multipart download
func (d *Downloader) downloadLargeFile(ctx context.Context, downloader *s3manager.Downloader, bucket string, key *string, localPath string) error {
	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file '%s': %w", aws.StringValue(key), err)
	}
	defer f.Close()

	downloadCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	_, err = downloader.DownloadWithContext(downloadCtx, f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    key,
	})

	if err != nil {
		os.Remove(localPath) // Clean up partially downloaded file
		return fmt.Errorf("failed to download '%s': %w", aws.StringValue(key), err)
	}

	return nil
}

// ListPrefixes lists prefixes (subdirectories) within a given S3 bucket and prefix
func (d *Downloader) ListPrefixes(bucket, prefix string) ([]string, error) {
	var prefixes []string
	err := d.s3.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String("/"),
		Prefix:    aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, p := range page.CommonPrefixes {
			prefixes = append(prefixes, aws.StringValue(p.Prefix))
		}
		return !lastPage
	})

	return prefixes, err
}
