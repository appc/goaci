package main

import (
	"archive/tar"
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
	assets    StringVector
	keepTmp   bool
	project   string
}

func getOptions() (*options, error) {
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

	// --asset
	flag.Var(&opts.assets, "asset", "Additional assets, can be used multiple times; format: <path in ACI>"+ListSeparator()+"<local path>; placeholders like <GOPATH> and <PROJPATH> can be used there as well")

	// --keep-tmp
	flag.BoolVar(&opts.keepTmp, "keep-tmp", false, "Do not delete temporary directory used for creating ACI")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		return nil, fmt.Errorf("Expected exactly one project to build, got %d", len(args))
	}
	opts.project = args[0]
	if opts.goBinary == "" {
		return nil, fmt.Errorf("Go binary not found")
	}

	return opts, nil
}

type pathsAndNames struct {
	tmpDirPath  string
	goPath      string
	goRootPath  string
	projectPath string
	fakeGoPath  string
	goBinPath   string
	aciDirPath  string
	rootFSPath  string
	goExecPath  string

	imageFileName string
	imageACName   string
}

// getGoPath returns go path and fake go path. The former is either in
// /tmp (which is a default) or some other path as specified by
// --go-path parameter. The latter is always in /tmp.
func getGoPath(opts *options, tmpDir string) (string, string) {
	fakeGoPath := filepath.Join(tmpDir, "gopath")
	if opts.goPath == "" {
		return fakeGoPath, fakeGoPath
	}
	return opts.goPath, fakeGoPath
}

// getNamesFromProject returns project name, image ACName and ACI
// filename. Names depend on whether project has several binaries. For
// project with single binary (github.com/appc/goaci) returned values
// would be: github.com/appc/goaci, github.com/appc/goaci and
// goaci.aci. For project with multiple binaries
// (github.com/appc/spec/...) returned values would be (assuming ace
// as selected binary): github.com/appc/spec, github.com/appc/spec-ace
// and spec-ace.aci.
func getNamesFromProject(opts *options) (string, string, string) {
	imageACName := opts.project
	projectName := imageACName
	base := filepath.Base(imageACName)
	threeDotsBase := base == "..."
	if threeDotsBase {
		imageACName = filepath.Dir(imageACName)
		projectName = imageACName
		base = filepath.Base(imageACName)
		if opts.useBinary != "" {
			suffix := "-" + opts.useBinary
			base += suffix
			imageACName += suffix
		}
	}

	return projectName, imageACName, base + schema.ACIExtension
}

func getPathsAndNames(opts *options) (*pathsAndNames, error) {
	tmpDir, err := ioutil.TempDir("", "goaci")
	if err != nil {
		return nil, fmt.Errorf("error setting up temporary directory: %v", err)
	}

	goPath, fakeGoPath := getGoPath(opts, tmpDir)
	projectName, imageACName, imageFileName := getNamesFromProject(opts)

	if os.Getenv("GOPATH") != "" {
		Warn("GOPATH env var is ignored, use --go-path=\"$GOPATH\" option instead")
	}
	goRoot := os.Getenv("GOROOT")
	if goRoot != "" {
		Warn("Overriding GOROOT env var to ", goRoot)
	}

	aciDir := filepath.Join(tmpDir, "aci")
	// Project name is path-like string with slashes, but slash is
	// not a file separator on every OS.
	projectPath := filepath.Join(goPath, "src", filepath.Join(strings.Split(projectName, "/")...))

	return &pathsAndNames{
		tmpDirPath:  tmpDir, // /tmp/XXX
		goPath:      goPath,
		goRootPath:  goRoot,
		projectPath: projectPath,
		fakeGoPath:  fakeGoPath,                       // /tmp/XXX/gopath
		goBinPath:   filepath.Join(fakeGoPath, "bin"), // /tmp/XXX/gopath/bin
		aciDirPath:  aciDir,                           // /tmp/XXX/aci
		rootFSPath:  filepath.Join(aciDir, "rootfs"),  // /tmp/XXX/aci/rootfs
		goExecPath:  opts.goBinary,

		imageFileName: imageFileName,
		imageACName:   imageACName,
	}, nil
}

func makeDirectories(pathsNames *pathsAndNames) error {
	// /tmp/XXX already exists, not creating it here

	// /tmp/XXX/gopath
	if err := os.Mkdir(pathsNames.fakeGoPath, 0755); err != nil {
		return err
	}

	// /tmp/XXX/gopath/bin
	if err := os.Mkdir(pathsNames.goBinPath, 0755); err != nil {
		return err
	}

	// /tmp/XXX/aci
	if err := os.Mkdir(pathsNames.aciDirPath, 0755); err != nil {
		return err
	}

	// /tmp/XXX/aci/rootfs
	if err := os.Mkdir(pathsNames.rootFSPath, 0755); err != nil {
		return err
	}

	return nil
}

func runGoGet(opts *options, pathsNames *pathsAndNames) error {
	// Construct args for a go get that does a static build
	args := []string{
		pathsNames.goExecPath,
		"get",
		"-a",
		"-tags", "netgo",
		"-ldflags", "'-w'",
		"-installsuffix", "nocgo", // for 1.4
		opts.project,
	}

	env := []string{
		"GOPATH=" + pathsNames.goPath,
		"GOBIN=" + pathsNames.goBinPath,
		"CGO_ENABLED=0",
		"PATH=" + os.Getenv("PATH"),
	}
	if pathsNames.goRootPath != "" {
		env = append(env, "GOROOT="+pathsNames.goRootPath)
	}

	cmd := exec.Cmd{
		Env:    env,
		Path:   pathsNames.goExecPath,
		Args:   args,
		Stderr: os.Stderr,
		Stdout: os.Stdout,
	}
	Debug("env: ", cmd.Env)
	Debug("running command: ", strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// getBinaryName get a binary name built by go get and selected by
// --use-binary parameter.
func getBinaryName(opts *options, pathsNames *pathsAndNames) (string, error) {
	fi, err := ioutil.ReadDir(pathsNames.goBinPath)
	if err != nil {
		return "", err
	}

	switch {
	case len(fi) < 1:
		return "", fmt.Errorf("No binaries found in gobin.")
	case len(fi) == 1:
		name := fi[0].Name()
		if opts.useBinary != "" && name != opts.useBinary {
			return "", fmt.Errorf("No such binary found in gobin: %q. There is only %q", opts.useBinary, name)
		}
		Debug("found binary: ", name)
		return name, nil
	case len(fi) > 1:
		names := []string{}
		for _, v := range fi {
			names = append(names, v.Name())
		}
		if opts.useBinary == "" {
			return "", fmt.Errorf("Found multiple binaries in gobin, but --use-binary option is not used. Please specify which binary to put in ACI. Following binaries are available: \"%s\"", strings.Join(names, `", "`))
		}
		for _, v := range names {
			if v == opts.useBinary {
				return v, nil
			}
		}
		return "", fmt.Errorf("No such binary found in gobin: %q. There are following binaries available: \"%s\"", opts.useBinary, strings.Join(names, `", "`))
	}
	return "", fmt.Errorf("Reaching this point shouldn't be possible.")
}

func getApp(opts *options, binary string) *types.App {
	exec := []string{filepath.Join("/", binary)}
	exec = append(exec, opts.exec...)

	return &types.App{
		Exec:  exec,
		User:  "0",
		Group: "0",
	}
}

func getVCSLabel(pathsNames *pathsAndNames) (*types.Label, error) {
	name, value, err := GetVCSInfo(pathsNames.projectPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to get VCS info: %v", err)
	}
	acname, err := types.NewACName(name)
	if err != nil {
		return nil, fmt.Errorf("Invalid VCS label: %v", err)
	}
	return &types.Label{
		Name:  *acname,
		Value: value,
	}, nil
}

func prepareManifest(opts *options, pathsNames *pathsAndNames, binary string) (*schema.ImageManifest, error) {
	name, err := types.NewACName(pathsNames.imageACName)
	// TODO(jonboulle): could this ever actually happen?
	if err != nil {
		return nil, err
	}

	app := getApp(opts, binary)

	vcsLabel, err := getVCSLabel(pathsNames)
	if err != nil {
		return nil, err
	}
	labels := types.Labels{
		*vcsLabel,
	}

	manifest := schema.BlankImageManifest()
	manifest.Name = *name
	manifest.App = app
	manifest.Labels = labels
	return manifest, nil
}

func copyAssets(opts *options, pathsNames *pathsAndNames) error {
	placeholderMapping := map[string]string{
		"<PROJPATH>": pathsNames.projectPath,
		"<GOPATH>":   pathsNames.goPath,
	}
	if err := PrepareAssets(opts.assets, pathsNames.rootFSPath, placeholderMapping); err != nil {
		return err
	}
	return nil
}

func moveBinaryToRootFS(pathsNames *pathsAndNames, binary string) error {
	// Move the binary into the rootfs
	ep := filepath.Join(pathsNames.rootFSPath, binary)
	if err := os.Rename(filepath.Join(pathsNames.goBinPath, binary), ep); err != nil {
		return err
	}
	Debug("moved binary to: ", ep)
	return nil
}

func writeACI(pathsNames *pathsAndNames, manifest *schema.ImageManifest) error {
	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	of, err := os.OpenFile(pathsNames.imageFileName, mode, 0644)
	if err != nil {
		return fmt.Errorf("Error opening output file: %v", err)
	}
	defer of.Close()

	gw := gzip.NewWriter(of)
	defer gw.Close()

	tr := tar.NewWriter(gw)
	defer tr.Close()

	// FIXME: the files in the tar archive are added with the
	// wrong uid/gid. The uid/gid of the aci builder leaks in the
	// tar archive. See: #16
	iw := aci.NewImageWriter(*manifest, tr)
	if err := filepath.Walk(pathsNames.aciDirPath, aci.BuildWalker(pathsNames.aciDirPath, iw)); err != nil {
		return err
	}
	if err := iw.Close(); err != nil {
		return err
	}
	Info("Wrote ", of.Name())
	return nil
}

func mainWithError() error {
	InitDebug()

	opts, err := getOptions()
	if err != nil {
		return err
	}

	pathsNames, err := getPathsAndNames(opts)
	if err != nil {
		return err
	}

	if opts.keepTmp {
		Info(`Preserving temporary directory "`, pathsNames.tmpDirPath, `"`)
	} else {
		defer os.RemoveAll(pathsNames.tmpDirPath)
	}

	if err := makeDirectories(pathsNames); err != nil {
		return err
	}

	if err := runGoGet(opts, pathsNames); err != nil {
		return err
	}

	binary, err := getBinaryName(opts, pathsNames)
	if err != nil {
		return err
	}

	manifest, err := prepareManifest(opts, pathsNames, binary)
	if err != nil {
		return err
	}

	if err := copyAssets(opts, pathsNames); err != nil {
		return err
	}

	if err := moveBinaryToRootFS(pathsNames, binary); err != nil {
		return err
	}

	if err := writeACI(pathsNames, manifest); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := mainWithError(); err != nil {
		Warn(err)
		os.Exit(1)
	}
}
