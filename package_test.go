package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type output struct {
	bytes.Buffer
}

func (output) Close() error {
	return nil
}

var update = flag.Bool("update", false, "update the golden test files")

func Test_buildPackage(t *testing.T) {

	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "success"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			files := make(chan FileStat)
			tarPackage := &output{}
			go func() {
				fstat1, _ := os.Stat("./testdata/example_dir/file1")

				files <- FileStat{
					baseDir:      "./testdata/example_dir/",
					relativePath: "file1",
					stat:         fstat1,
					err:          nil,
				}
				fstat2, _ := os.Stat("./testdata/example_dir/dir1/file2")
				files <- FileStat{
					baseDir:      "./testdata/example_dir/",
					relativePath: "dir1/file2",
					stat:         fstat2,
					err:          nil,
				}
				fstat3, _ := os.Stat("./testdata/example_dir/symlink1")
				files <- FileStat{
					baseDir:      "./testdata/example_dir/",
					relativePath: "symlink1",
					link:         "file1",
					stat:         fstat3,
					err:          nil,
				}
				close(files)
			}()

			if err := buildPackage("site", tarPackage, files); (err != nil) != tt.wantErr {
				t.Errorf("buildPackage() error: '%v', wantErr: '%v'", err, tt.wantErr)
			}

			gp := filepath.Join("testdata", t.Name()+".golden.tar.gz")

			if *update {
				t.Log("update golden file")
				if err := ioutil.WriteFile(gp, tarPackage.Bytes(), 0644); err != nil {
					t.Fatalf("failed to update golden file: %s", err)
				}
			}

			g, err := ioutil.ReadFile(gp)
			if err != nil {
				t.Fatalf("failed reading .golden: %s", err)
			}

			if !bytes.Equal(tarPackage.Bytes(), g) {
				t.Errorf("buildPackage() tar output does not match %s file", gp)
			}

		})
	}
}
