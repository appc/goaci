package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var debugEnabled bool
var pathListSep string

func printTo(w io.Writer, i ...interface{}) {
	s := fmt.Sprint(i...)
	fmt.Fprintln(w, strings.TrimSuffix(s, "\n"))
}

func Warn(i ...interface{}) {
	printTo(os.Stderr, i...)
}

func Info(i ...interface{}) {
	printTo(os.Stdout, i...)
}

func Debug(i ...interface{}) {
	if debugEnabled {
		printTo(os.Stdout, i...)
	}
}

func InitDebug() {
	if os.Getenv("GOACI_DEBUG") != "" {
		debugEnabled = true
	}
}

// ListSeparator returns filepath.ListSeparator rune as a string.
func ListSeparator() string {
	if pathListSep == "" {
		len := utf8.RuneLen(filepath.ListSeparator)
		if len < 0 {
			panic("filepath.ListSeparator is not valid utf8?!")
		}
		buf := make([]byte, len)
		len = utf8.EncodeRune(buf, filepath.ListSeparator)
		pathListSep = string(buf[:len])
	}

	return pathListSep
}
