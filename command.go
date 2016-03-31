// Copyright 2016 The appc Authors.
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

package main

// command provides an interface for named actions for command line
// purposes.
type command interface {
	// Name should return a name of a command usable at command
	// line.
	Name() string
	// Run should parse given args and perform some action. name
	// parameter is given for usage purposes.
	Run(name string, args []string) error
}
