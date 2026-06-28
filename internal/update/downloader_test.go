package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchBytesRejectsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	downloader := NewDownloader(time.Second, nil)
	_, err := downloader.FetchBytes(context.Background(), server.URL, 128, true)
	if err == nil {
		t.Fatal("expected status error")
	}
}

func TestFetchBytesRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("too large"))
	}))
	defer server.Close()
	downloader := NewDownloader(time.Second, nil)
	_, err := downloader.FetchBytes(context.Background(), server.URL, 3, true)
	if err == nil {
		t.Fatal("expected oversize error")
	}
}

func TestDownloadArtifactVerifiesSizeAndHash(t *testing.T) {
	body := []byte("package")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer server.Close()
	downloader := NewDownloader(time.Second, nil)
	path, err := downloader.DownloadArtifact(context.Background(), server.URL+"/pkg.zip", t.TempDir(), int64(len(body)), SHA256BytesHex(body), true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "pkg.zip") {
		t.Fatalf("unexpected path %s", path)
	}
	_, err = downloader.DownloadArtifact(context.Background(), server.URL+"/pkg.zip", t.TempDir(), int64(len(body)+1), SHA256BytesHex(body), true)
	if err == nil {
		t.Fatal("expected size mismatch")
	}
	_, err = downloader.DownloadArtifact(context.Background(), server.URL+"/pkg.zip", t.TempDir(), int64(len(body)), strings.Repeat("0", 64), true)
	if err == nil {
		t.Fatal("expected sha mismatch")
	}
}

func TestDownloaderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("late"))
	}))
	defer server.Close()
	downloader := NewDownloader(10*time.Millisecond, nil)
	_, err := downloader.FetchBytes(context.Background(), server.URL, 128, true)
	if err == nil {
		t.Fatal("expected timeout")
	}
}
