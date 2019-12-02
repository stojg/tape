package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

func buildPackage(app application, writer io.WriteCloser, files <-chan FileStat) error {

	destBaseDir := app.tarPrefix
	defer closer(writer)

	gw, err := gzip.NewWriterLevel(writer, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer closer(gw)

	tw := tar.NewWriter(gw)
	defer closer(tw)

	var compressed int64
	var size int64
	for file := range files {
		written, err := addFile(tw, destBaseDir, file)
		if err != nil {
			return err
		}
		size = size + written
		compressed++
	}
	app.Debugf("%d files (%s) were compressed into a tar\n", compressed, byteCountDecimal(size))
	return nil
}

func addFile(tw *tar.Writer, destBaseDir string, file FileStat) (int64, error) {

	header, err := tar.FileInfoHeader(file.stat, file.link)
	if err != nil {
		return 0, fmt.Errorf("FileInfoHeader: %s", err)
	}

	// tweak the Name inside the tar so that get tar:ed out properly
	header.Name = filepath.Join(destBaseDir, file.relativePath)

	// write the header to the tarball archive
	if err := tw.WriteHeader(header); err != nil {
		return 0, fmt.Errorf("WriteHeader: %s", err)
	}

	// return on non-regular files, no other data to copy into the tarball
	if !file.stat.Mode().IsRegular() {
		return 0, nil
	}

	f, err := os.Open(path.Join(file.baseDir, file.relativePath))
	if err != nil {
		return 0, fmt.Errorf("cant open file: %s", err)
	}
	defer closer(f)

	// copy the file data to the tarball
	n, err := io.Copy(tw, f)
	if err != nil {
		return n, fmt.Errorf("can't copy file data into tarball: %s", err)
	}
	return n, nil
}
