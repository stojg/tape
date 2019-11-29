package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func scanDirectory(baseDir string) (<-chan FileStat, error) {

	out := make(chan FileStat)

	stat, err := os.Stat(baseDir)
	if os.IsNotExist(err) {
		return out, fmt.Errorf("path '%s' doesn't exist", baseDir)
	}
	if err != nil {
		return out, err
	}

	if !stat.IsDir() {
		return out, fmt.Errorf("%s is not a directory", baseDir)
	}

	go func() {
		defer close(out)
		err := filepath.Walk(baseDir, func(filePath string, stat os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			link := ""
			if stat.Mode()&os.ModeSymlink != 0 {
				if link, err = os.Readlink(filePath); err != nil {
					return err
				}
			}

			// get path relative to baseDir
			relPath, err := filepath.Rel(baseDir, filePath)
			if err != nil {
				return err
			}

			out <- FileStat{
				baseDir:      baseDir,
				relativePath: relPath,
				stat:         stat,
				link:         link,
			}
			return nil
		})
		if err != nil {
			out <- FileStat{err: err}
		}
	}()
	return out, nil
}
