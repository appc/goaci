// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proj2aci

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

// DirExists checks if directory exists if given path is not empty.
//
// This function is rather specific as it is mostly used for checking
// overrides validity (like overriding temporary directory, where
// empty string means "do not override").
func DirExists(path string) bool {
	if path != "" {
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			return false
		}
	}
	return true
}

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

// listSeparator returns filepath.ListSeparator rune as a string.
func listSeparator() string {
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
