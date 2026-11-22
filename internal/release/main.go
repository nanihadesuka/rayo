// Command release packs what build.sh --release built, the same on any system: a .tar.gz or .zip for
// each, with the license, and SHA256SUMS.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := pack(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// A time of its own, or each build would differ.
var always = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

type file struct {
	name string
	mode int64
	data []byte
}

func pack(dist string) error {
	license, err := os.ReadFile("LICENSE")
	if err != nil {
		return err
	}
	license = bytes.ReplaceAll(license, []byte("\r\n"), []byte("\n"))
	folders, err := os.ReadDir(dist)
	if err != nil {
		return err
	}
	var sums strings.Builder
	for _, d := range folders {
		folder := filepath.Join(dist, d.Name())
		program, archive, write := "rayo", d.Name()+".tar.gz", tarGz
		if strings.Contains(d.Name(), "windows") {
			program, archive, write = "rayo.exe", d.Name()+".zip", zipped
		}
		data, err := os.ReadFile(filepath.Join(folder, program))
		if err != nil {
			return err
		}
		var b bytes.Buffer
		if err := write(&b, []file{{program, 0o755, data}, {"LICENSE", 0o644, license}}); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dist, archive), b.Bytes(), 0o644); err != nil {
			return err
		}
		if err := os.RemoveAll(folder); err != nil {
			return err
		}
		fmt.Printf("%-28s %5.1f MB, %.1f MB to download\n", archive, float64(len(data))/1e6, float64(b.Len())/1e6)
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(b.Bytes()), archive)
	}
	return os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte(sums.String()), 0o644)
}

func tarGz(w io.Writer, files []file) error {
	z, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	t := tar.NewWriter(z)
	for _, f := range files {
		if err := t.WriteHeader(&tar.Header{Name: f.name, Mode: f.mode, Size: int64(len(f.data)), ModTime: always}); err != nil {
			return err
		}
		if _, err := t.Write(f.data); err != nil {
			return err
		}
	}
	if err := t.Close(); err != nil {
		return err
	}
	return z.Close()
}

func zipped(w io.Writer, files []file) error {
	z := zip.NewWriter(w)
	z.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) { return flate.NewWriter(w, flate.BestCompression) })
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: always}
		h.SetMode(os.FileMode(f.mode))
		part, err := z.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := part.Write(f.data); err != nil {
			return err
		}
	}
	return z.Close()
}
