package engine

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

type CompressionType int

const (
	CompNone CompressionType = iota
	CompGzip
	CompXz
)

func detectCompression(path string) CompressionType {
	switch {
	case strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz"):
		return CompGzip
	case strings.HasSuffix(path, ".tar.xz") || strings.HasSuffix(path, ".txz"):
		return CompXz
	case strings.HasSuffix(path, ".gz"):
		return CompGzip
	case strings.HasSuffix(path, ".xz"):
		return CompXz
	default:
		return CompNone
	}
}

func decompressReader(r io.Reader, comp CompressionType) (io.Reader, error) {
	switch comp {
	case CompGzip:
		return gzip.NewReader(r)
	case CompXz:
		return xz.NewReader(r)
	default:
		return r, nil
	}
}

func tarArchive(srcPath string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		tw := tar.NewWriter(pw)
		defer tw.Close()

		filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(filepath.Dir(srcPath), path)
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relPath)

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(tw, f)
			return err
		})
	}()

	return pr, nil
}

func extractTar(reader io.Reader, destPath string) error {
	tr := tar.NewReader(reader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}

	return nil
}

func ExtractArchive(path string, destPath string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", path, err)
	}
	defer f.Close()

	comp := detectCompression(path)
	reader, err := decompressReader(f, comp)
	if err != nil {
		return fmt.Errorf("failed to decompress %s: %w", path, err)
	}

	return extractTar(reader, destPath)
}

func isTarArchive(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".gz", ".tgz", ".xz", ".txz":
		inner := strings.TrimSuffix(path, ext)
		if extInner := filepath.Ext(inner); extInner == ".tar" {
			return true
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	comp := detectCompression(path)
	reader, err := decompressReader(f, comp)
	if err != nil {
		return false
	}

	buf := make([]byte, 262)
	n, err := io.ReadFull(reader, buf)
	if n < 262 {
		return false
	}
	_ = err

	return strings.Contains(string(buf[:n]), "ustar")
}
