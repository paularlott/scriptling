package pack

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultMaxPackageSize int64         = 100 * 1024 * 1024 // 100MB
	DefaultCacheTTL                     = 7 * 24 * time.Hour // 7 days
)

// IsURL returns true if source starts with http:// or https://.
func IsURL(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}

// splitHash separates a source string from an optional #sha256=<hex> fragment.
// Returns the clean source and the expected hash (empty if none).
func splitHash(source string) (string, string) {
	if i := strings.LastIndex(source, "#sha256="); i != -1 {
		return source[:i], source[i+8:]
	}
	return source, ""
}

// HashBytes returns the SHA-256 hex digest of data.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Fetch loads bytes from a URL or local path.
// For URLs, uses the cache with ETag/Last-Modified freshness checks.
func Fetch(source string, insecure bool) ([]byte, error) {
	return FetchWithCache(source, insecure, "")
}

// FetchWithCache loads bytes from a URL or local path, using cacheDir for remote URLs.
// If cacheDir is empty, uses the OS default cache directory.
// An optional #sha256=<hex> fragment on source is stripped before fetching and
// used to verify the downloaded bytes; a mismatch is a fatal error.
// maxSize limits download size (0 = use DefaultMaxPackageSize).
func FetchWithCache(source string, insecure bool, cacheDir string, maxSize ...int64) ([]byte, error) {
	source, expectedHash := splitHash(source)
	limit := DefaultMaxPackageSize
	if len(maxSize) > 0 && maxSize[0] > 0 {
		limit = maxSize[0]
	}
	var data []byte
	var err error
	if IsURL(source) {
		data, err = fetchURLCached(source, insecure, cacheDir, limit)
	} else {
		data, err = FetchFile(source, limit)
	}
	if err != nil {
		return nil, err
	}
	if expectedHash != "" {
		if got := HashBytes(data); got != expectedHash {
			return nil, fmt.Errorf("package hash mismatch for %s: expected %s got %s", source, expectedHash, got)
		}
	}
	return data, nil
}

// DefaultCacheDir returns the default cache directory for packages.
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine cache directory: %w", err)
	}
	return filepath.Join(base, "scriptling", "packages"), nil
}

// ClearCache removes all cached packages from cacheDir.
// If cacheDir is empty, uses the OS default cache directory.
func ClearCache(cacheDir string) error {
	if cacheDir == "" {
		var err error
		cacheDir, err = DefaultCacheDir()
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil // nothing to clear
	}
	return os.RemoveAll(cacheDir)
}

// PruneCache removes cache entries that have not been accessed within ttl.
// If cacheDir is empty, uses the OS default cache directory.
// If ttl is 0, uses DefaultCacheTTL.
// Each cache entry is a .zip/.meta pair; the .zip mod time tracks last access.
func PruneCache(cacheDir string, ttl time.Duration) error {
	if cacheDir == "" {
		var err error
		cacheDir, err = DefaultCacheDir()
		if err != nil {
			return nil // no cache dir, nothing to prune
		}
	}
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := time.Now().Add(-ttl)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".zip" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			base := strings.TrimSuffix(filepath.Join(cacheDir, e.Name()), ".zip")
			_ = os.Remove(base + ".zip")
			_ = os.Remove(base + ".meta")
		}
	}
	return nil
}

// FetchFile loads from local filesystem.
func FetchFile(path string, maxSize ...int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrFetchFailed, path, err)
	}
	defer f.Close()
	limit := DefaultMaxPackageSize
	if len(maxSize) > 0 && maxSize[0] > 0 {
		limit = maxSize[0]
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrFetchFailed, path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %s: exceeds maximum size of %d bytes", ErrFetchFailed, path, limit)
	}
	return data, nil
}

// fetchURLCached downloads from HTTP/HTTPS, using a disk cache with ETag/Last-Modified validation.
func fetchURLCached(url string, insecure bool, cacheDir string, limit int64) ([]byte, error) {
	if cacheDir == "" {
		var err error
		cacheDir, err = DefaultCacheDir()
		if err != nil {
			// Fall back to direct download if cache dir unavailable
			return fetchURLDirect(url, insecure, nil)
		}
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fetchURLDirect(url, insecure, nil)
	}

	client := httpClient(insecure)
	key := urlCacheKey(url)
	dataFile := filepath.Join(cacheDir, key+".zip")
	metaFile := filepath.Join(cacheDir, key+".meta")

	// Read cached metadata (etag\nlast-modified)
	etag, lastMod := readCacheMeta(metaFile)

	// If we have a cached copy, do a conditional HEAD request
	if _, err := os.Stat(dataFile); err == nil && (etag != "" || lastMod != "") {
		req, err := http.NewRequest(http.MethodHead, url, nil)
		if err == nil {
			if etag != "" {
				req.Header.Set("If-None-Match", etag)
			}
			if lastMod != "" {
				req.Header.Set("If-Modified-Since", lastMod)
			}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusNotModified {
					// Cache is fresh — touch mod time so TTL resets on access
					now := time.Now()
					_ = os.Chtimes(dataFile, now, now)
					return os.ReadFile(dataFile)
				}
			}
		}
	}

	// Download fresh copy
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrFetchFailed, url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrFetchFailed, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s: HTTP %d", ErrFetchFailed, url, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrFetchFailed, url, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %s: exceeds maximum size of %d bytes", ErrFetchFailed, url, limit)
	}

	// Write to cache
	_ = os.WriteFile(dataFile, data, 0644)
	newEtag := resp.Header.Get("ETag")
	newLastMod := resp.Header.Get("Last-Modified")
	if newEtag != "" || newLastMod != "" {
		_ = os.WriteFile(metaFile, []byte(newEtag+"\n"+newLastMod), 0644)
	}

	return data, nil
}

// fetchURLDirect downloads without caching.
func fetchURLDirect(url string, insecure bool, client *http.Client) ([]byte, error) {
	if client == nil {
		client = httpClient(insecure)
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrFetchFailed, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s: HTTP %d", ErrFetchFailed, url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, DefaultMaxPackageSize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrFetchFailed, url, err)
	}
	if int64(len(data)) > DefaultMaxPackageSize {
		return nil, fmt.Errorf("%w: %s: exceeds maximum size of %d bytes", ErrFetchFailed, url, DefaultMaxPackageSize)
	}
	return data, nil
}

func httpClient(insecure bool) *http.Client {
	if insecure {
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		}
	}
	return &http.Client{}
}

// urlCacheKey returns a stable filename-safe key for a URL.
func urlCacheKey(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:])
}

// readCacheMeta reads etag and last-modified from a meta file.
func readCacheMeta(path string) (etag, lastMod string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(data), "\n", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(string(data)), ""
}
