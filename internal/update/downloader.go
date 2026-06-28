package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultMaxManifestBytes = 1 << 20
	DefaultMaxPackageBytes  = 512 << 20
)

type Downloader struct {
	Client           *http.Client
	Logger           *slog.Logger
	MaxManifestBytes int64
	MaxPackageBytes  int64
}

func NewDownloader(timeout time.Duration, logger *slog.Logger) *Downloader {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	client.CheckRedirect = boundedRedirects(5)
	return &Downloader{
		Client:           client,
		Logger:           logger,
		MaxManifestBytes: DefaultMaxManifestBytes,
		MaxPackageBytes:  DefaultMaxPackageBytes,
	}
}

func boundedRedirects(max int) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= max {
			return fmt.Errorf("too many redirects")
		}
		if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme == "http" {
			return fmt.Errorf("refusing HTTPS to HTTP redirect")
		}
		return nil
	}
}

func (d *Downloader) FetchSignedManifest(ctx context.Context, manifestURL string, publicKey ed25519.PublicKey) (*Manifest, []byte, error) {
	raw, err := d.FetchBytes(ctx, manifestURL, d.MaxManifestBytes, true)
	if err != nil {
		return nil, nil, err
	}
	sig, err := d.FetchBytes(ctx, manifestURL+".sig", ed25519SignatureSize, true)
	if err != nil {
		return nil, nil, err
	}
	manifest, err := ParseVerifiedManifest(raw, sig, publicKey)
	if err != nil {
		return nil, nil, err
	}
	return manifest, raw, nil
}

const ed25519SignatureSize = 64

func (d *Downloader) FetchBytes(ctx context.Context, rawURL string, maxBytes int64, allowHTTP bool) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxManifestBytes
	}
	if err := validateURL(rawURL, allowHTTP); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "warband-vault-updater/1")
	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("download %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	return body, nil
}

func (d *Downloader) DownloadArtifact(ctx context.Context, rawURL, downloadDir string, expectedSize int64, expectedSHA256 string, allowHTTP bool) (string, error) {
	if expectedSize <= 0 {
		return "", fmt.Errorf("expected size must be positive")
	}
	if expectedSize > d.MaxPackageBytes {
		return "", fmt.Errorf("expected package size exceeds maximum %d", d.MaxPackageBytes)
	}
	if !isSHA256Hex(expectedSHA256) {
		return "", fmt.Errorf("expected sha256 is invalid")
	}
	if err := validateURL(rawURL, allowHTTP); err != nil {
		return "", err
	}
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", fmt.Errorf("create downloads directory: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("create artifact request: %w", err)
	}
	req.Header.Set("User-Agent", "warband-vault-updater/1")
	resp, err := d.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download artifact: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("artifact download returned HTTP %d", resp.StatusCode)
	}
	base := filepath.Base(req.URL.Path)
	if base == "." || base == "/" || base == "" {
		base = "update-package"
	}
	tmp, err := os.OpenFile(filepath.Join(downloadDir, base+".partial"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("create partial download: %w", err)
	}
	tmpPath := tmp.Name()
	finalPath := strings.TrimSuffix(tmpPath, ".partial")
	hasher := sha256.New()
	limited := io.LimitReader(resp.Body, expectedSize+1)
	written, copyErr := io.Copy(io.MultiWriter(tmp, hasher), limited)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write partial download: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close partial download: %w", closeErr)
	}
	if written > expectedSize {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("artifact exceeds expected size %d", expectedSize)
	}
	if written != expectedSize {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("artifact size mismatch: expected %d got %d", expectedSize, written)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, expectedSHA256) {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("artifact sha256 mismatch: expected %s got %s", expectedSHA256, actual)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("finish download: %w", err)
	}
	if d.Logger != nil {
		d.Logger.Info("download verified", "path", finalPath, "bytes", written)
	}
	return finalPath, nil
}

func validateURL(rawURL string, allowHTTP bool) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http") {
		return fmt.Errorf("URL must use HTTPS")
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL host is required")
	}
	return nil
}
