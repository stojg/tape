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

func buildPackage(destBaseDir string, writer io.WriteCloser, files <-chan FileStat) error {

	defer closer(writer)

	// set up the gzip writer, BestSpeed is okay since the biggest files are typically images and other binary assets
	gw, err := gzip.NewWriterLevel(writer, gzip.BestSpeed)
	if err != nil {
		return err
	}
	defer closer(gw)

	tw := tar.NewWriter(gw)
	defer closer(tw)

	var compressed uint64
	for file := range files {
		if err := addFile(tw, destBaseDir, file); err != nil {
			return err
		}
		compressed++
	}

	fmt.Printf("[-] %d files were compressed into a tar archive\n", compressed)
	return nil
}

func addFile(tw *tar.Writer, destBaseDir string, file FileStat) error {

	header, err := tar.FileInfoHeader(file.stat, file.link)
	if err != nil {
		return fmt.Errorf("FileInfoHeader: %s", err)
	}

	// tweak the Name inside the tar so that get tar:ed out properly
	header.Name = filepath.Join(destBaseDir, file.relativePath)

	// write the header to the tarball archive
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("WriteHeader: %s", err)
	}

	// return on non-regular files, no other data to copy into the tarball
	if !file.stat.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(path.Join(file.baseDir, file.relativePath))
	if err != nil {
		return fmt.Errorf("cant open file: %s", err)
	}
	defer closer(f)

	// copy the file data to the tarball
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("can't copy file data into tarball: %s", err)
	}

	return nil
}
