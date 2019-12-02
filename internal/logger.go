package internal

import (
	"fmt"
	"os"
	"strings"
)

type Logger interface {
	Infof(string, ...interface{})
	Errorf(string, ...interface{})
	Debugf(string, ...interface{})
}

func NewCliLogger(debug bool) Logger {
	return &CliLogger{debug: debug}
}

type CliLogger struct {
	debug bool
}

func (l CliLogger) Infof(format string, a ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	_, _ = fmt.Fprintf(os.Stdout, "[-] "+format, a...)
}

func (l CliLogger) Errorf(format string, a ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	_, _ = fmt.Fprintf(os.Stdout, format, a...)
}

func (l CliLogger) Debugf(format string, a ...interface{}) {
	if !l.debug {
		return
	}
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	_, _ = fmt.Fprintf(os.Stdout, "    "+format, a...)
}
