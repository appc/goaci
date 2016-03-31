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

package proj2aci

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type CmdFailedError struct {
	Err error
}

func (e CmdFailedError) Error() string {
	return fmt.Sprintf("CmdFailedError: %s", e.Err.Error())
}

type CmdNotFoundError struct {
	Err error
}

func (e CmdNotFoundError) Error() string {
	return fmt.Sprintf("CmdNotFoundError: %s", e.Err.Error())
}

// RunCmdFull runs given execProg. execProg should be an absolute path
// to a program or it can be an empty string. In the latter case first
// string in args is taken and searched for in $PATH.
//
// If execution fails then CmdFailedError is returned. This can be
// useful if we don't care if execution fails or not. CmdNotFoundError
// is returned if executable is not found.
func RunCmdFull(execProg string, args, env []string, cwd string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("No args to execute passed")
	}
	prog := execProg
	if prog == "" {
		pathProg, err := exec.LookPath(args[0])
		if err != nil {
			return CmdNotFoundError{err}
		}
		prog = pathProg
	} else if _, err := os.Stat(prog); err != nil {
		return CmdNotFoundError{err}
	}
	cmd := exec.Cmd{
		Path:   prog,
		Args:   args,
		Env:    env,
		Dir:    cwd,
		Stdout: stdout,
		Stderr: stderr,
	}
	Debug(`running command: "`, strings.Join(args, `" "`), `"`)
	if err := cmd.Run(); err != nil {
		return CmdFailedError{err}
	}
	return nil
}

func RunCmd(args, env []string, cwd string) error {
	return RunCmdFull("", args, env, cwd, os.Stdout, os.Stderr)
}
