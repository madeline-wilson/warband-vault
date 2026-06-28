package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type ExtractionOptions struct {
	MaxFiles            int
	MaxExpandedBytes    int64
	ExpectedExecutables []string
}

func DefaultExtractionOptions() ExtractionOptions {
	return ExtractionOptions{
		MaxFiles:         2048,
		MaxExpandedBytes: 768 << 20,
	}
}

func ExtractArchive(ctx context.Context, archivePath, stagingDir string, opts ExtractionOptions) error {
	if opts.MaxFiles <= 0 {
		opts.MaxFiles = 2048
	}
	if opts.MaxExpandedBytes <= 0 {
		opts.MaxExpandedBytes = 768 << 20
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		if err := extractZip(ctx, archivePath, stagingDir, opts); err != nil {
			return err
		}
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		if err := extractTarGZ(ctx, archivePath, stagingDir, opts); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported archive format %s", archivePath)
	}
	for _, executable := range opts.ExpectedExecutables {
		path, err := safeJoin(stagingDir, executable)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("expected executable %s is absent: %w", executable, err)
		}
		if info.IsDir() {
			return fmt.Errorf("expected executable %s is a directory", executable)
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(path, 0o755); err != nil {
				return fmt.Errorf("set executable permissions on %s: %w", executable, err)
			}
		}
	}
	return nil
}

func extractZip(ctx context.Context, archivePath, stagingDir string, opts ExtractionOptions) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > opts.MaxFiles {
		return fmt.Errorf("archive contains too many files")
	}
	seen := map[string]struct{}{}
	var expanded int64
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		mode := file.FileInfo().Mode()
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("archive contains symlink %s", file.Name)
		}
		if !mode.IsRegular() && !file.FileInfo().IsDir() {
			return fmt.Errorf("archive contains unsupported file %s", file.Name)
		}
		dest, key, err := safeArchivePath(stagingDir, file.Name)
		if err != nil {
			return err
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("archive contains duplicate path %s", file.Name)
		}
		seen[key] = struct{}{}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return fmt.Errorf("create directory from archive: %w", err)
			}
			continue
		}
		expanded += int64(file.UncompressedSize64)
		if expanded > opts.MaxExpandedBytes {
			return fmt.Errorf("archive expands beyond %d bytes", opts.MaxExpandedBytes)
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip member %s: %w", file.Name, err)
		}
		if err := writeArchiveFile(dest, rc, mode.Perm()); err != nil {
			rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return fmt.Errorf("close zip member %s: %w", file.Name, err)
		}
	}
	return nil
}

func extractTarGZ(ctx context.Context, archivePath, stagingDir string, opts ExtractionOptions) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open tar.gz archive: %w", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seen := map[string]struct{}{}
	var count int
	var expanded int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}
		count++
		if count > opts.MaxFiles {
			return fmt.Errorf("archive contains too many files")
		}
		dest, key, err := safeArchivePath(stagingDir, header.Name)
		if err != nil {
			return err
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("archive contains duplicate path %s", header.Name)
		}
		seen[key] = struct{}{}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return fmt.Errorf("create directory from archive: %w", err)
			}
		case tar.TypeReg:
			expanded += header.Size
			if expanded > opts.MaxExpandedBytes {
				return fmt.Errorf("archive expands beyond %d bytes", opts.MaxExpandedBytes)
			}
			if err := writeArchiveFile(dest, io.LimitReader(tr, header.Size), os.FileMode(header.Mode).Perm()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("archive contains unsupported entry %s", header.Name)
		}
	}
	return nil
}

func safeArchivePath(root, name string) (string, string, error) {
	dest, err := safeJoin(root, name)
	if err != nil {
		return "", "", err
	}
	clean := filepath.ToSlash(filepath.Clean(name))
	return dest, strings.ToLower(clean), nil
}

func safeJoin(root, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("archive path is empty")
	}
	normalizedSeparators := strings.ReplaceAll(name, `\`, `/`)
	if strings.HasPrefix(name, `\\`) || filepath.IsAbs(name) || filepath.VolumeName(name) != "" || hasWindowsDriveName(name) || strings.HasPrefix(normalizedSeparators, "../") || strings.Contains(normalizedSeparators, "/../") {
		return "", fmt.Errorf("archive path %q is absolute", name)
	}
	clean := filepath.Clean(normalizedSeparators)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || strings.HasPrefix(filepath.ToSlash(clean), "../") {
		return "", fmt.Errorf("archive path %q escapes staging directory", name)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve staging directory: %w", err)
	}
	dest := filepath.Join(rootAbs, clean)
	rel, err := filepath.Rel(rootAbs, dest)
	if err != nil {
		return "", fmt.Errorf("verify archive path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive path %q escapes staging directory", name)
	}
	return dest, nil
}

func hasWindowsDriveName(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	c := name[0]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func writeArchiveFile(dest string, src io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create archive parent directory: %w", err)
	}
	if mode == 0 {
		mode = 0o644
	}
	if mode&0o111 != 0 && runtime.GOOS != "windows" {
		mode = 0o755
	} else {
		mode = 0o644
	}
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create archive file %s: %w", dest, err)
	}
	defer file.Close()
	if _, err := io.Copy(file, src); err != nil {
		return fmt.Errorf("write archive file %s: %w", dest, err)
	}
	return nil
}
