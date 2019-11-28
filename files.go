package main

import (
	"fmt"
	"os"
	"path/filepath"
)

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
		fmt.Println("[-] scanning directory for compression")
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
		fmt.Println("[-] directory scan completed")

	}()
	return out, nil
}
