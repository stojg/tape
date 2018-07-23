package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func buildPackage(tarDirectoryName string, writer io.WriteCloser, files chan FileStat) error {

	defer writer.Close()

	// set up the gzip writer, BestSpeed is okay since the biggest files are typically images and other binary assets
	gw, err := gzip.NewWriterLevel(writer, gzip.BestSpeed)
	if err != nil {
		return err
	}
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	compressed := 0
	for file := range files {
		if err := addFile(tw, tarDirectoryName, file); err != nil {
			return err
		}
		compressed++
	}

	fmt.Printf("[-] %d files were compressed into a tar archive\n", compressed)
	return nil
}

func addFile(tw *tar.Writer, tarDirectoryName string, file FileStat) error {

	// update the name to correctly reflect the desired destination when untaring
	relativeName := strings.TrimPrefix(strings.Replace(file.path, file.src, "", -1), string(filepath.Separator))

	// root folder
	if file.path == file.src {
		return nil
	}

	header, err := tar.FileInfoHeader(file.stat, file.link)
	if err != nil {
		return err
	}

	// tweak the Name inside the tar so that get tar:ed out properly
	header.Name = filepath.Join(tarDirectoryName, relativeName)

	// write the header to the tarball archive
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// return on non-regular files, no other data to copy into the tarball
	if !file.stat.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(file.path)
	if err != nil {
		return err
	}
	defer f.Close()

	// copy the file data to the tarball
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("can't copy file data into tarball: %s", err)
	}

	return nil
}
