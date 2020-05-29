package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// moduleName takes a packageName and attempts to figure out the module name. It
// does so by looking in $GOPATH/pkg/mod and gradually stripping away parts
// (ex /foo) from the packageName until it finds a directory (module) that
// matches.
//
// TODO: This is hacky - it'd be much nicer if there was a library that figured
// this out holistically, or perhaps by querying module proxy.
func moduleName(packageName string) string {
	parts := strings.Split(packageName, "/")

	for i := range parts {
		modulePathParts := append([]string{build.Default.GOPATH, "pkg", "mod"}, parts[0:len(parts)-1-i]...)
		fi, err := os.Stat(filepath.Join(modulePathParts...))
		if _, ok := err.(*os.PathError); ok {
			// If this isn't a directory - keep going!
			continue
		}
		if err != nil {
			// Ok this is actually a real error.
			panic(err)
		}
		if fi.IsDir() {
			mn := strings.Join(parts[0:len(parts)-1-i], "/")
			if mn == "" {
				// Probably stdlib (something like "io" or "io/ioutil").
				return "stdlib"
			}
			return mn
		}
	}

	panic(errors.New("this should not have happened..."))
}

// moduleFiles finds all the files of a module (given the module's root path).
// TODO(deklerk): We need to copy the $GOPATH/mod/pkg to /tmp first, since
// $GOPATH is protected by -mod=readonly.
func moduleFiles(moduleRootPath string) []string {
	// Copy $GOPATH/mod/pkg/moduleRootPath to /tmp because we can't run
	// `go list` in $GOPATH due to -mod=readonly restriction.
	tmpdir, err := ioutil.TempDir("", "list-tmp")
	if err != nil {
		panic(err)
	}

	newDir := tmpdir + "/listme"
	cpCMD := exec.Command("cp", "-R", moduleRootPath, newDir)
	cpCMD.Stderr = os.Stderr
	if err := cpCMD.Run(); err != nil {
		panic(err)
	}

	// Set permissions so that we can modify go.mod / go.sum. (well, so that
	// go list can)
	//
	// NOTE: This won't work on windows, will it? 0777 looks very linux
	// specific.
	if err := os.Chmod(newDir, 0777); err != nil {
		panic(err)
	}
	if err := os.Chmod(filepath.Join(newDir, "go.mod"), 0777); err != nil {
		panic(err)
	}
	touchCMD := exec.Command("touch", "go.sum")
	touchCMD.Stderr = os.Stderr
	touchCMD.Dir = newDir
	if err := touchCMD.Run(); err != nil {
		panic(err)
	}
	if err := os.Chmod(filepath.Join(newDir, "go.sum"), 0777); err != nil {
		panic(err)
	}

	// Run `go list` in tmp dir.
	type GoList struct {
		Dir         string   `json:"Dir"`
		GoFiles     []string `json:"GoFiles"`
		TestGoFiles []string `json:"XTestGoFiles"`
	}

	listCMD := exec.Command("go", "list", "-json", "./...")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	listCMD.Stdout = &stdout
	listCMD.Stderr = &stderr
	listCMD.Dir = newDir
	if err := listCMD.Run(); err != nil {
		panic(fmt.Errorf("failed to run `cd %s && go list -json ./...`:\n%s\n%v", newDir, stderr.String(), err))
	}

	var files []string
	dec := json.NewDecoder(&stdout)
	for {
		var outDecoded GoList

		err := dec.Decode(&outDecoded)
		if err == io.EOF {
			// all done
			break
		}
		if err != nil {
			panic(err)
		}

		for _, f := range outDecoded.GoFiles {
			files = append(files, filepath.Join(outDecoded.Dir, f))
		}

		for _, f := range outDecoded.TestGoFiles {
			files = append(files, filepath.Join(outDecoded.Dir, f))
		}
	}

	return files
}

var gopathDir = build.Default.GOPATH

func ModuleSize(module string) (int64, error) {
	path := attemptToFindModuleOnFS(module)
	if path == "" {
		return -1, nil
	}

	var size int64
	if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	}); err != nil {
		return -1, err
	}
	return size, nil
}

var versionRegexp = regexp.MustCompile("(.+)/v[0-9]+")

func attemptToFindModuleOnFS(module string) string {
	module = replaceCapitalLetters(module)

	if loc := findOnFS(module); loc != "" {
		return loc
	}

	if err := goGet(module); err != nil {
		panic(err)
	}

	return findOnFS(module)
}

func findOnFS(module string) string {
	gopathAttempt := filepath.Join(build.Default.GOPATH, "pkg", "mod", module)
	if _, err := os.Stat(gopathAttempt); err == nil {
		return gopathAttempt
	}

	curdir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	curModuleRootName, curModuleRootPath := getModuleRoot(curdir)
	if curModuleRootName == module {
		return curModuleRootPath
	}

	// TODO: try vendor

	return ""
}

func getModuleRoot(dir string) (name, root string) {
	type Module struct {
		Path string `json:"Path"`
		Dir  string `json:"Dir"`
	}

	type GoListOutput struct {
		Mod Module `json:"Module"`
	}

	decoded := GoListOutput{}

	cmd := exec.Command("go", "list", "-json", "./...")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("failed to run `cd %s && go list -json ./...`:\n%s\n%v", dir, stderr.String(), err))
	}

	if err := json.NewDecoder(&stdout).Decode(&decoded); err != nil {
		panic(err)
	}

	return decoded.Mod.Path, decoded.Mod.Dir
}

// goGet caches the given module.
func goGet(moduleName string) error {
	// Use a tempdir so that go get doesn't modify the current go.mod.
	dir := os.TempDir()

	// Get the module, causing it to be cached.
	cmd := exec.Command("go", "get", moduleName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir

	// So that HOME is set, from which GOCACHE can be inferred.
	cmd.Env = os.Environ()

	// So that we don't get "path@version can't be used in GOPATH mode" errors.
	cmd.Env = append(cmd.Env, "GO111MODULE=on")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running `go get %s`: %s", moduleName, err)
	}

	return nil
}
