package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// ProgressFunc is called with download progress
type ProgressFunc func(downloaded, total int64)

// Downloader handles downloading update files
type Downloader struct {
	httpClient *http.Client
	logger     *slog.Logger
}

// DownloadResult contains the downloaded file information
type DownloadResult struct {
	Path   string
	Size   int64
	SHA256 string
}

// NewDownloader creates a new downloader
func NewDownloader(logger *slog.Logger) *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		logger: logger,
	}
}

// Download downloads a file from the given URL to the destination path
func (d *Downloader) Download(ctx context.Context, url string, dest string, progress ProgressFunc) (*DownloadResult, error) {
	d.logger.Info("downloading update",
		"url", url,
		"dest", dest,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "nametag-updater/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Create destination file
	file, err := os.Create(dest)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	// Create hash writer
	hash := sha256.New()

	// Create multi-writer to write to both file and hash
	writer := io.MultiWriter(file, hash)

	// Track progress
	var downloaded int64
	total := resp.ContentLength

	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{
			reader: resp.Body,
			onProgress: func(n int64) {
				downloaded += n
				progress(downloaded, total)
			},
		}
	}

	// Copy data
	size, err := io.Copy(writer, reader)
	if err != nil {
		os.Remove(dest)
		return nil, fmt.Errorf("copy: %w", err)
	}

	hashSum := hex.EncodeToString(hash.Sum(nil))

	d.logger.Info("download complete",
		"size", size,
		"sha256", hashSum,
	)

	return &DownloadResult{
		Path:   dest,
		Size:   size,
		SHA256: hashSum,
	}, nil
}

// VerifyChecksum verifies that a file matches the expected SHA256 hash
func VerifyChecksum(filePath string, expectedSHA256 string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actual)
	}

	return nil
}

// progressReader wraps an io.Reader and calls onProgress for each read
type progressReader struct {
	reader     io.Reader
	onProgress func(n int64)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	if n > 0 {
		p.onProgress(int64(n))
	}
	return n, err
}
