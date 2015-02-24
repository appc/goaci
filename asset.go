package main

import (
	"path/filepath"
	"fmt"
	"io"
	"os"
	"strings"
)

func copyTree(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rootLess := path[len(src):]
		target := filepath.Join(dest, rootLess)
		mode := info.Mode()
		switch {
		case mode.IsDir():
			err := os.Mkdir(target, mode.Perm())
			if err != nil {
				return err
			}
		case mode.IsRegular():
			srcFile, err := os.Open(path)
			if err != nil {
				return err
			}
			destFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(destFile, srcFile); err != nil {
				return err
			}
		case mode&os.ModeSymlink == os.ModeSymlink:
			// TODO(krnowak): preserve absolute paths of symlinks
			symTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(symTarget) {
				dirPart := filepath.Dir(path)
				testPath := filepath.Join(dirPart, symTarget)
				symTarget = filepath.Clean(testPath)
			}
			if strings.HasPrefix(symTarget, src) {
				relTarget, err := filepath.Rel(filepath.Dir(path), symTarget)
				if err != nil {
					return err
				}
				if err := os.Symlink(relTarget, target); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("Symlink %s points to %s, which is outside asset %s", path, symTarget, src)
			}
		default:
			return fmt.Errorf("Unsupported node (%s) in assets, only regular files, directories and symlinks pointing to node inside asset are supported.", path, mode.String())
		}
		return nil
	})
}

// TODO(krnowak): Add placeholders - <PROJDIR>, <TMPDIR>, <GOPATH>?
// First one for sure will be useful, maybe GOPATH too, not sure about
// TMPDIR, rather not.
func PrepareAssets(assets []string, rootfs string) error {
	for _, asset := range assets {
		splitAsset := filepath.SplitList(asset)
		if len(splitAsset) != 2 {
			return fmt.Errorf("Malformed asset option: '%v' - expected two absolute paths separated with %v", asset, listSeparator())
		}
		ACIAsset := splitAsset[0]
		localAsset := splitAsset[1]
		if !filepath.IsAbs(ACIAsset) {
			return fmt.Errorf("Malformed asset option: '%v' - ACI asset has to be absolute path", asset)
		}
		if !filepath.IsAbs(localAsset) {
			return fmt.Errorf("Malformed asset option: '%v' - local asset has to be absolute path", asset)
		}
		fi, err := os.Stat(localAsset)
		if err != nil {
			return fmt.Errorf("Error stating %v: %v", localAsset, err)
		}
		if fi.Mode().IsDir() || fi.Mode().IsRegular() {
			ACIBase := filepath.Base(ACIAsset)
			ACIAssetSubPath := filepath.Join(rootfs, filepath.Dir(ACIAsset))
			err := os.MkdirAll(ACIAssetSubPath, 0755)
			if err != nil {
				return fmt.Errorf("Failed to create directory tree for asset '%v': %v", asset, err)
			}
			err = copyTree(localAsset, filepath.Join(ACIAssetSubPath, ACIBase))
			if err != nil {
				return fmt.Errorf("Failed to copy assets for '%v': %v", asset, err)
			}
		} else {
			return fmt.Errorf("Can't handle %v - not a file, not a dir", fi.Name())
		}
	}
	return nil
}
