package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, entries map[string][]byte, symlink string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pkg.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if symlink != "" {
		header := &zip.FileHeader{Name: symlink, Method: zip.Deflate}
		header.SetMode(os.ModeSymlink | 0o777)
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("target"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTarGZ(t *testing.T, name string, body []byte, typeflag byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pkg.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	header := &tar.Header{Name: name, Size: int64(len(body)), Mode: 0o644, Typeflag: typeflag}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if typeflag == tar.TypeReg {
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractZipAcceptsExpectedExecutable(t *testing.T) {
	archive := writeZip(t, map[string][]byte{"warband-vault": []byte("bin")}, "")
	staging := t.TempDir()
	opts := DefaultExtractionOptions()
	opts.ExpectedExecutables = []string{"warband-vault"}
	if err := ExtractArchive(context.Background(), archive, staging, opts); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	archive := writeZip(t, map[string][]byte{"../evil": []byte("x")}, "")
	if err := ExtractArchive(context.Background(), archive, t.TempDir(), DefaultExtractionOptions()); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestExtractZipRejectsAbsolutePath(t *testing.T) {
	archive := writeZip(t, map[string][]byte{"/tmp/evil": []byte("x")}, "")
	if err := ExtractArchive(context.Background(), archive, t.TempDir(), DefaultExtractionOptions()); err == nil {
		t.Fatal("expected absolute path error")
	}
}

func TestExtractZipRejectsSymlink(t *testing.T) {
	archive := writeZip(t, map[string][]byte{}, "link")
	if err := ExtractArchive(context.Background(), archive, t.TempDir(), DefaultExtractionOptions()); err == nil {
		t.Fatal("expected symlink error")
	}
}

func TestExtractCorruptArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	if err := os.WriteFile(path, []byte("not zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ExtractArchive(context.Background(), path, t.TempDir(), DefaultExtractionOptions()); err == nil {
		t.Fatal("expected corrupt archive error")
	}
}

func TestExtractMissingExecutable(t *testing.T) {
	archive := writeZip(t, map[string][]byte{"other": []byte("x")}, "")
	opts := DefaultExtractionOptions()
	opts.ExpectedExecutables = []string{"warband-vault"}
	if err := ExtractArchive(context.Background(), archive, t.TempDir(), opts); err == nil {
		t.Fatal("expected missing executable error")
	}
}

func TestExtractTarRejectsTraversalAndSymlink(t *testing.T) {
	traversal := writeTarGZ(t, "../evil", []byte("x"), tar.TypeReg)
	if err := ExtractArchive(context.Background(), traversal, t.TempDir(), DefaultExtractionOptions()); err == nil {
		t.Fatal("expected traversal error")
	}
	symlink := writeTarGZ(t, "link", []byte{}, tar.TypeSymlink)
	if err := ExtractArchive(context.Background(), symlink, t.TempDir(), DefaultExtractionOptions()); err == nil {
		t.Fatal("expected symlink error")
	}
}

func TestSafeJoinRejectsWindowsDrive(t *testing.T) {
	if _, err := safeJoin(t.TempDir(), `C:\evil`); err == nil {
		t.Fatal("expected drive-letter path error")
	}
}

func TestTarGZValidFile(t *testing.T) {
	archive := writeTarGZ(t, "warband-vault", []byte("bin"), tar.TypeReg)
	opts := DefaultExtractionOptions()
	opts.ExpectedExecutables = []string{"warband-vault"}
	if err := ExtractArchive(context.Background(), archive, t.TempDir(), opts); err != nil {
		t.Fatal(err)
	}
}
