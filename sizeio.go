package main

import (
	"fmt"
	"io"
	"sync/atomic"
)

type SizeReader struct {
	r    io.ReadCloser
	n    int64
	done bool
}

func NewSizeReader(r io.ReadCloser) *SizeReader {
	return &SizeReader{
		r: r,
	}
}
func (r *SizeReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	atomic.AddInt64(&r.n, int64(n))
	if err != nil && err == io.EOF {
		r.done = true
	}
	return
}

func (r *SizeReader) Close() error {
	return r.r.Close()
}

// N gets the number of bytes that have been read
// so far.
func (r *SizeReader) N() (int64, bool) {
	return atomic.LoadInt64(&r.n), r.done
}

type SizeWriter struct {
	w io.WriteCloser
	n int64
}

func NewSizeWriter(r io.WriteCloser) *SizeWriter {
	return &SizeWriter{
		w: r,
	}
}
func (w *SizeWriter) Write(p []byte) (n int, err error) {
	n, err = w.w.Write(p)
	atomic.AddInt64(&w.n, int64(n))
	return
}

func (w *SizeWriter) Close() error {
	return w.w.Close()
}

// N gets the number of bytes that have been written so far.
func (w *SizeWriter) N() int64 {
	return atomic.LoadInt64(&w.n)
}

func byteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
