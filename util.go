package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var debugEnabled bool
var pathListSep string

func warn(s string, i ...interface{}) {
	s = fmt.Sprintf(s, i...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(s, "\n"))
}

func die(s string, i ...interface{}) {
	warn(s, i...)
	os.Exit(1)
}

func debug(i ...interface{}) {
	if debugEnabled {
		s := fmt.Sprint(i...)
		fmt.Fprintln(os.Stderr, strings.TrimSuffix(s, "\n"))
	}
}

func initDebug() {
	if os.Getenv("GOACI_DEBUG") != "" {
		debugEnabled = true
	}
}

func listSeparator() string {
	if pathListSep == "" {
		len := utf8.RuneLen(filepath.ListSeparator)
		if len < 0 {
			die("list separator is not valid utf8?!")
		}
		buf := make([]byte, len)
		len = utf8.EncodeRune(buf, filepath.ListSeparator)
		pathListSep = string(buf[:len])
	}

	return pathListSep
}
