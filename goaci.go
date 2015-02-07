package main

// TODO(jonboulle): at a bare minimum, allow user to specify arguments to exec
// TODO(jonboulle): allow user to add assets to the ACI
// TODO(jonboulle): support user-specified GOPATHs/local packages. Right now we pull down a fresh copy of the specified package every time. This is better in terms of isolation and reproducibility, but inconvenient.
// TODO(jonboulle): add git SHA as a label in the image manifest
// TODO(jonboulle): support passing user-supplied arguments to `go get`? this might be tricky as we need to set a lot ourselves, and what if they conflict?
// TODO(jonboulle): support multiple executables?

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

var Debug bool

func die(s string, i ...interface{}) {
	s = fmt.Sprintf(s, i...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(s, "\n"))
	os.Exit(1)
}

func debug(i ...interface{}) {
	if Debug {
		s := fmt.Sprint(i...)
		fmt.Fprintln(os.Stderr, strings.TrimSuffix(s, "\n"))
	}
}

func main() {
	if os.Getenv("GOPATH") != "" {
		die("to avoid confusion GOPATH must not be set")
	}
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		die("GOROOT must be set")
	}
	if os.Getenv("GOACI_DEBUG") != "" {
		Debug = true
	}

	// Set up a temporary directory for everything (gopath and builds)
	tmpdir, err := ioutil.TempDir("", "goaci")
	if err != nil {
		die("error setting up temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Scratch build dir for aci
	acidir := filepath.Join(tmpdir, "aci")

	// Be explicit with gobin
	gobin := filepath.Join(tmpdir, "bin")

	// Find the go binary
	gocmd, err := exec.LookPath("go")
	if err != nil {
		die("could not find `go` in path")
	}

	// Construct args for a go get that does a static build
	// TODO(jonboulle): go version 1.4
	args := []string{
		gocmd,
		"get",
		"-a",
		"-tags", "netgo",
		"-ldflags", "'-w'",
	}

	// Extract the package name (which is the last arg).
	var ns string
	for _, arg := range os.Args[1:] {
		// TODO(jonboulle): try to pass the other args on to go get?
		//		args = append(args, arg)
		ns = arg
	}

	name, err := types.NewACName(ns)
	// TODO(jonboulle): could this ever actually happen?
	if err != nil {
		die("bad app name: %v", err)
	}

	// Use the last component, e.g. example.com/my/app --> app
	ofn := filepath.Base(ns) + ".aci"
	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	of, err := os.OpenFile(ofn, mode, 0644)
	if err != nil {
		die("error opening output file: %v", err)
	}

	cmd := exec.Cmd{
		Env: []string{
			"GOPATH=" + tmpdir,
			"GOBIN=" + gobin,
			"GOROOT=" + goroot,
			"CGO_ENABLED=0",
			"PATH=" + os.Getenv("PATH"),
		},
		Path:   gocmd,
		Args:   args,
		Stderr: os.Stderr,
		Stdout: os.Stdout,
	}
	debug("env:", cmd.Env)
	debug("running command:", strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		die("error running go: %v", err)
	}

	// Check that we got 1 binary from the go get command
	fi, err := ioutil.ReadDir(gobin)
	if err != nil {
		die(err.Error())
	}
	switch {
	case len(fi) < 1:
		die("no binaries found in gobin")
	case len(fi) > 1:
		debug(fmt.Sprint(fi))
		die("can't handle multiple binaries")
	}
	fn := fi[0].Name()
	debug("found binary: ", fn)

	// Set up rootfs for ACI layout
	rfs := filepath.Join(acidir, "rootfs")
	err = os.MkdirAll(rfs, 0755)
	if err != nil {
		die(err.Error())
	}

	// Move the binary into the rootfs
	ep := filepath.Join(rfs, fn)
	err = os.Rename(filepath.Join(gobin, fn), ep)
	if err != nil {
		die(err.Error())
	}
	debug("moved binary to:", ep)

	// Build the ACI
	im := schema.ImageManifest{
		ACKind:    types.ACKind("ImageManifest"),
		ACVersion: schema.AppContainerVersion,
		Name:      *name,
		App: &types.App{
			Exec: types.Exec{
				filepath.Join("/", fn),
			},
			User:  "0",
			Group: "0",
		},
	}
	debug(im)

	gw := gzip.NewWriter(of)
	tr := tar.NewWriter(gw)

	defer func() {
		tr.Close()
		gw.Close()
		of.Close()
	}()

	iw := aci.NewImageWriter(im, tr)
	err = filepath.Walk(acidir, aci.BuildWalker(acidir, iw))
	if err != nil {
		die(err.Error())
	}
	err = iw.Close()
	if err != nil {
		die(err.Error())
	}
	fmt.Println("Wrote", of.Name())
}

// strip replaces all characters that are not [a-Z_] with _
func strip(in string) string {
	out := bytes.Buffer{}
	for _, c := range in {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTVWXYZ0123456789_", c) {
			c = '_'
		}
		if _, err := out.WriteRune(c); err != nil {
			panic(err)
		}
	}
	return out.String()
}
