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

func main() {

	if len(os.Args) != 3 {
		fmt.Printf("usage: %s path/to/src/directory destination.tar.gz\n", os.Args[0])
		os.Exit(1)
	}

	basePath, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	files, err := scanDirectory(basePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = buildPackage("site", os.Args[2], files)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type FileStat struct {
	src  string
	path string
	link string
	stat os.FileInfo
	err  error
}

func scanDirectory(absPath string) (chan FileStat, error) {

	out := make(chan FileStat)

	stat, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return out, fmt.Errorf("path '%s' doesn't exist", absPath)
	}
	if err != nil {
		return out, err
	}

	if !stat.IsDir() {
		return out, fmt.Errorf("%s is not a directory", absPath)
	}

	go func() {
		defer close(out)

		err := filepath.Walk(absPath, func(filePath string, stat os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			link := ""
			if stat.Mode()&os.ModeSymlink != 0 {
				if link, err = os.Readlink(filePath); err != nil {
					return err
				}
			}

			out <- FileStat{
				src:  absPath,
				path: filePath,
				stat: stat,
				link: link,
			}
			return nil
		})
		if err != nil {
			out <- FileStat{err: err}
		}

	}()

	return out, nil
}

func buildPackage(tarDirectoryName string, dest string, files chan FileStat) error {
	file, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer file.Close()
	// set up the gzip writer, BestSpeed is okay since the biggest files are typically images and other binary assets
	gw, err := gzip.NewWriterLevel(file, gzip.BestSpeed)
	if err != nil {
		return err
	}
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for file := range files {
		if err := addFile(tw, tarDirectoryName, file); err != nil {
			return err
		}
	}
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
