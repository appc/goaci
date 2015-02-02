package main

// TODO(jonboulle): use
// TODO(jonboulle): support user-specified GOPATHs?
// TODO(jonboulle): multiple executables?
// TODO(jonboulle): add git sha label

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/pkg/tarheader"
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

	// Set up a temporary directory for the build
	gopath, err := ioutil.TempDir("", "goaci")
	if err != nil {
		die("error setting up temporary directory: %v", err)
	}
	defer os.RemoveAll(gopath)
	acidir := filepath.Join(gopath, "aci")
	// Be explicit with gobin
	gobin := filepath.Join(gopath, "bin")

	// Find the go binary
	gocmd, err := exec.LookPath("go")
	if err != nil {
		die("could not find `go` in path")
	}

	// Construct args for a go get that does a static build
	args := []string{
		gocmd,
		"get",
		"-a",
		"-tags", "netgo",
		"-ldflags", "'-w'",
	}

	// Extract the package name (which is the last arg).
	// Try to pass the other args on to `go get`
	var ns string
	for _, arg := range os.Args[1:] {
		args = append(args, arg)
		ns = arg
	}

	name, err := types.NewACName(ns)
	// TODO(jonboulle): could this ever actually happen?
	if err != nil {
		die("bad app name: %v", err)
	}

	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	of, err := os.OpenFile(strip(ns), mode, 0644)
	if err != nil {
		die("error opening output file: %v", err)
	}

	cmd := exec.Cmd{
		Env: []string{
			"GOPATH=" + gopath,
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
	err = filepath.Walk(acidir, buildWalker(acidir, iw))
	if err != nil {
		die(err.Error())
	}
	err = iw.Close()
	if err != nil {
		die(err.Error())
	}
}

/*
type ImageManifest struct {
	ACKind        types.ACKind       `json:"acKind"`
	ACVersion     types.SemVer       `json:"acVersion"`
	Name          types.ACName       `json:"name"`
	Labels        types.Labels       `json:"labels,omitempty"`
	App           *types.App         `json:"app,omitempty"`
	Annotations   types.Annotations  `json:"annotations,omitempty"`
	Dependencies  types.Dependencies `json:"dependencies,omitempty"`
	PathWhitelist []string           `json:"pathWhitelist,omitempty"`
}
*/

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

// TODO(jonboulle): share this with actool/build
func buildWalker(root string, aw aci.ArchiveWriter) filepath.WalkFunc {
	// cache of inode -> filepath, used to leverage hard links in the archive
	inos := map[uint64]string{}
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relpath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relpath == "." {
			return nil
		}
		if relpath == aci.ManifestFile {
			// ignore; this will be written by the archive writer
			// TODO(jonboulle): does this make sense? maybe just remove from archivewriter?
			return nil
		}

		link := ""
		var r io.Reader
		switch info.Mode() & os.ModeType {
		case os.ModeCharDevice:
		case os.ModeDevice:
		case os.ModeDir:
		case os.ModeSymlink:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			link = target
		default:
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			r = file
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			panic(err)
		}
		// Because os.FileInfo's Name method returns only the base
		// name of the file it describes, it may be necessary to
		// modify the Name field of the returned header to provide the
		// full path name of the file.
		hdr.Name = relpath
		tarheader.Populate(hdr, info, inos)
		// If the file is a hard link to a file we've already seen, we
		// don't need the contents
		if hdr.Typeflag == tar.TypeLink {
			hdr.Size = 0
			r = nil
		}
		if err := aw.AddFile(hdr, r); err != nil {
			return err
		}

		return nil
	}
}
