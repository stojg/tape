package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func scanDirectory(app application) (<-chan FileStat, error) {

	out := make(chan FileStat)

	stat, err := os.Stat(app.baseDir)
	if os.IsNotExist(err) {
		return out, fmt.Errorf("path '%s' doesn't exist", app.baseDir)
	}
	if err != nil {
		return out, err
	}

	if !stat.IsDir() {
		return out, fmt.Errorf("%s is not a directory", app.baseDir)
	}

	go func(a application) {
		defer close(out)
		a.Debugf("starting file scan of %s", app.baseDir)
		err := filepath.Walk(app.baseDir, func(filePath string, stat os.FileInfo, err error) error {
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
			relPath, err := filepath.Rel(app.baseDir, filePath)
			if err != nil {
				return err
			}

			out <- FileStat{
				baseDir:      app.baseDir,
				relativePath: relPath,
				stat:         stat,
				link:         link,
			}
			return nil
		})

		if err != nil {
			out <- FileStat{err: err}
		}
	}(app)
	return out, nil
}
