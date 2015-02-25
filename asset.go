package main

import (
	"path/filepath"
	"fmt"
	"io"
	"os"
	"strings"
)

func copyTree(src, dest, imageAssetDir string) error {
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
			symTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			absolute := true
			if !filepath.IsAbs(symTarget) {
				absolute = false
				dirPart := filepath.Dir(path)
				testPath := filepath.Join(dirPart, symTarget)
				symTarget = filepath.Clean(testPath)
			}
			if strings.HasPrefix(symTarget, src) {
				var err error;
				linkTarget := ""
				if absolute {
					linkTarget, err = filepath.Rel(src, symTarget)
					if err == nil {
						linkTarget = filepath.Join(imageAssetDir, linkTarget)
						linkTarget = filepath.Clean(linkTarget)
					}
				} else {
					linkTarget, err = filepath.Rel(filepath.Dir(path), symTarget)
				}
				if err != nil {
					return err
				}
				if err := os.Symlink(linkTarget, target); err != nil {
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

func replacePlaceholders(path string, paths map[string]string) string {
	fmt.Printf("Processing path: %s\n", path)
	newPath := path
	for placeholder, replacement := range paths {
		newPath = strings.Replace(newPath, placeholder, replacement, -1)
	}
	fmt.Printf("Processed path: %s\n", newPath)
	return newPath
}

func PrepareAssets(assets []string, rootfs string, paths map[string]string) error {
	for _, asset := range assets {
		splitAsset := filepath.SplitList(asset)
		if len(splitAsset) != 2 {
			return fmt.Errorf("Malformed asset option: '%v' - expected two absolute paths separated with %v", asset, listSeparator())
		}
		ACIAsset := replacePlaceholders(splitAsset[0], paths)
		localAsset := replacePlaceholders(splitAsset[1], paths)
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
			ACIAssetSubPath := filepath.Join(rootfs, filepath.Dir(ACIAsset))
			err := os.MkdirAll(ACIAssetSubPath, 0755)
			if err != nil {
				return fmt.Errorf("Failed to create directory tree for asset '%v': %v", asset, err)
			}
			err = copyTree(localAsset, filepath.Join(rootfs, ACIAsset), ACIAsset)
			if err != nil {
				return fmt.Errorf("Failed to copy assets for '%v': %v", asset, err)
			}
		} else {
			return fmt.Errorf("Can't handle %v - not a file, not a dir", fi.Name())
		}
	}
	return nil
}
