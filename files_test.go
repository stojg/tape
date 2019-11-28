package main

import (
	"testing"
)

func Test_scanDirectory(t *testing.T) {
	tests := []struct {
		name    string
		absPath string
		want    int
		wantErr bool
	}{
		{name: "dir_not_exists", absPath: "not-exist", want: 0, wantErr: true},
		{name: "not_a_dir", absPath: "./testdata/example_dir/file1", want: 0, wantErr: true},
		{name: "normal", absPath: "./testdata/example_dir/", want: 6, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := scanDirectory(tt.absPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("scanDirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			var files []FileStat
			for s := range got {
				files = append(files, s)
			}

			if len(files) != tt.want {
				t.Errorf("scanDirectory() expected %d files and directories, got %d", tt.want, len(files))
			}
		})
	}
}

func Benchmark_scanDirectory(b *testing.B) {

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		got, err := scanDirectory("./testdata")
		if err != nil {
			b.Fatal(err)
		}
		// sink
		for range got {
		}
	}
}
