package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
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

func warn(s string, i ...interface{}) {
	s = fmt.Sprintf(s, i...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(s, "\n"))
}

func die(s string, i ...interface{}) {
	warn(s, i...)
	os.Exit(1)
}

func debug(i ...interface{}) {
	if Debug {
		s := fmt.Sprint(i...)
		fmt.Fprintln(os.Stderr, strings.TrimSuffix(s, "\n"))
	}
}

type StringVector []string

func (v *StringVector) String() string {
	return `"` + strings.Join(*v, `" "`) + `"`
}

func (v *StringVector) Set(str string) error {
	*v = append(*v, str)
	return nil
}

type options struct {
	exec      StringVector
	goBinary  string
	goPath    string
	useBinary string
}

func getOptions() *options {
	opts := &options{}

	// --go-binary
	goDefaultBinaryDesc := "Go binary to use"
	gocmd, err := exec.LookPath("go")
	if err != nil {
		goDefaultBinaryDesc += " (default: none found in $PATH, so it must be provided)"
	} else {
		goDefaultBinaryDesc += " (default: whatever go in $PATH)"
	}
	flag.StringVar(&opts.goBinary, "go-binary", gocmd, goDefaultBinaryDesc)

	// --exec
	flag.Var(&opts.exec, "exec", "Parameters passed to app, can be used multiple times")

	// --go-path
	flag.StringVar(&opts.goPath, "go-path", "", "Custom GOPATH (default: a temporary directory)")

	// --use-binary
	flag.StringVar(&opts.useBinary, "use-binary", "", "Which executable to put in ACI image")
	flag.Parse()

	if opts.goBinary == "" {
		die("go binary not found")
	}

	return opts
}

func main() {
	opts := getOptions()
	if os.Getenv("GOPATH") != "" {
		warn("GOPATH env var is ignored, use --go-path=\"$GOPATH\" option instead")
	}
	goRoot := os.Getenv("GOROOT")
	if goRoot != "" {
		warn("Overriding GOROOT env var to %s", goRoot)
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
	goPath := opts.goPath
	if goPath == "" {
		goPath = tmpdir
	}

	// Scratch build dir for aci
	acidir := filepath.Join(tmpdir, "aci")

	// Let's put final binary in tmpdir
	gobin := filepath.Join(tmpdir, "bin")

	// Construct args for a go get that does a static build
	args := []string{
		opts.goBinary,
		"get",
		"-a",
		"-tags", "netgo",
		"-ldflags", "'-w'",
		// 1.4
		"-installsuffix", "cgo",
	}

	// Extract the package name (which is the last arg).
	var ns string
	for _, arg := range os.Args[1:] {
		// TODO(jonboulle): try to pass the other args on to go get?
		//		args = append(args, arg)
		ns = arg
	}

	// Use the last sensible component, e.g. example.com/my/app --> app
	// or example.com/my/app/... -> app
	// When using --use-binary=bin option, append binary name -> app-bin
	fullPkgName := ns
	base := filepath.Base(ns)
	if base == "..." {
		fullPkgName = filepath.Dir(ns)
		base = filepath.Base(fullPkgName)
	}
	if opts.useBinary != "" {
		suffix := "-" + opts.useBinary
		base += suffix
		fullPkgName += suffix
	}

	name, err := types.NewACName(fullPkgName)
	// TODO(jonboulle): could this ever actually happen?
	if err != nil {
		die("bad app name: %v", err)
	}
	args = append(args, ns)
	ofn := base + ".aci"
	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	of, err := os.OpenFile(ofn, mode, 0644)
	if err != nil {
		die("error opening output file: %v", err)
	}

	env := []string{
		"GOPATH=" + goPath,
		"GOBIN=" + gobin,
		"CGO_ENABLED=0",
		"PATH=" + os.Getenv("PATH"),
	}
	if goRoot != "" {
		env = append(env, "GOROOT="+goRoot)
	}
	cmd := exec.Cmd{
		Env:    env,
		Path:   opts.goBinary,
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
	var fn string
	switch {
	case len(fi) < 1:
		die("No binaries found in gobin.")
	case len(fi) == 1:
		name := fi[0].Name()
		if opts.useBinary != "" && name != opts.useBinary {
			die("No such binary found in gobin: %s. There is only %s", opts.useBinary, name)
		}
		fn = name
	case len(fi) > 1:
		names := []string{}
		for _, v := range fi {
			names = append(names, v.Name())
		}
		if opts.useBinary == "" {
			die("Found multiple binaries in gobin, but --use-binary option is not used. Please specify which binary to put in ACI. Following binaries are available: %s", strings.Join(names, ", "))
		}
		for _, v := range names {
			if v == opts.useBinary {
				fn = v
				break
			}
		}
		if fn == "" {
			die("No such binary found in gobin: %s. There are following binaries available: %s", opts.useBinary, strings.Join(names, ", "))
		}
	}
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

	exec := []string{filepath.Join("/", fn)}
	exec = append(exec, opts.exec...)
	// Build the ACI
	im := schema.ImageManifest{
		ACKind:    types.ACKind("ImageManifest"),
		ACVersion: schema.AppContainerVersion,
		Name:      *name,
		App: &types.App{
			Exec:  exec,
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
