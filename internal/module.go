package internal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type ModuleSizer struct{}

// ModuleSize returns the size of the module on the OS. It is non-cumulative.
//
// If the module can not be found, it returns -1,nil.
//
// Other errors are returned as -1,err.
func (ms *ModuleSizer) ModuleSize(module string) (int64, error) {
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

// moduleNameFromModulePath takes a modulePath and attempts to figure out the
// module name.
//
// For example, something like golang.org/x/text@v0.3.0 becomes
// golang.org/x/text.
//
// It panics if it encounters an error or can't figure out the module name.
//
// TODO: It'd be much nicer if there was a library that figured this out
// holistically, or perhaps by querying module proxy.
func moduleNameFromModulePath(modulePath string) string {
	parts := strings.Split(modulePath, "@")
	if len(parts) != 2 {
		panic(fmt.Sprintf("couldn't figure out module name from module path %s", modulePath))
	}
	return parts[0]
}

// moduleFiles finds all the files of a module (given the module's root path).
//
// TODO(deklerk): We need to copy the $GOPATH/mod/pkg to /tmp first, since
// $GOPATH is protected by -mod=readonly.
func moduleFiles(moduleRootPath string) (moduleFiles []string, cleanup func(), _ error) {
	tmpWorkDir, cleanup, err := readableModCache(moduleRootPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a readable copy of %s: %s", moduleRootPath, err)
	}

	// Run `go list` in tmp dir.
	type GoList struct {
		Dir          string   `json:"Dir"`
		GoFiles      []string `json:"GoFiles"`
		TestGoFiles  []string `json:"TestGoFiles"`
		XTestGoFiles []string `json:"XTestGoFiles"`
	}

	listCMD := exec.Command("go", "list", "-mod=readonly", "-json", "./...")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	listCMD.Stdout = &stdout
	listCMD.Stderr = &stderr
	listCMD.Dir = tmpWorkDir
	if err := listCMD.Run(); err != nil {
		return nil, nil, fmt.Errorf("failed to run `cd %s && go list -mod=readonly -json ./...`:\n%s\n%v", tmpWorkDir, stderr.String(), err)
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
			return nil, nil, fmt.Errorf("error decoding go list -json output: %s", err)
		}

		for _, f := range outDecoded.GoFiles {
			files = append(files, filepath.Join(outDecoded.Dir, f))
		}
		for _, f := range outDecoded.TestGoFiles {
			files = append(files, filepath.Join(outDecoded.Dir, f))
		}
		for _, f := range outDecoded.XTestGoFiles {
			files = append(files, filepath.Join(outDecoded.Dir, f))
		}
	}

	return files, cleanup, nil
}

var versionRegexp = regexp.MustCompile("(.+)/v[0-9]+")

// attemptToFindModuleOnFS attempts to find module on the fs. If it's found, it
// returns the filepath. If not, it returns empty string.
//
// As part of its attempts, it will `go get` missing modules.
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

// findOnFS checks whether the given module is in,
// - $GOPATH/pkg/mod/<module-path>
// - $CURDIR/
//
// If it finds the module at one of the above, it returns the filepath. If not,
// it returns empty.
//
// TODO: It should try $CURDIR/vendor, too.
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

	cmd := exec.Command("go", "list", "-mod=readonly", "-json", "./...")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("failed to run `cd %s && go list -mod=readonly -json ./...`:\n%s\n%v", dir, stderr.String(), err))
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
	cmd := exec.Command("go", "get", "-d", moduleName)
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

// indirectModules returns the // indirect module references in the given
// moduleRootPath's go.mod.
//
// TODO(deklerk): We need to copy the $GOPATH/mod/pkg to /tmp first, since
// $GOPATH is protected by -mod=readonly.
func indirectModules(moduleRootPath string) []string {
	tmpWorkDir, cleanup, err := readableModCache(moduleRootPath)
	if err != nil {
		panic(fmt.Errorf("failed to create a readable copy of %s: %s", moduleRootPath, err))
	}
	defer cleanup()

	// Run `go list` in tmp dir.
	type Module struct {
		Name     string `json:"name"`
		Indirect bool   `json:"indirect"`
	}

	cmd := exec.Command("go", "list", "-mod=readonly", "-f", `{"name":"{{.Path}}","indirect":{{.Indirect}}}`, "-m", "all")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = tmpWorkDir

	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf(`failed to run $(cd %s && go list -mod=readonly -f '{"name":"{{.Path}}","indirect":{{.Indirect}}}' -m all :\n%s\n%v}`, tmpWorkDir, stderr.String(), err))
	}

	var modules []Module
	b := bufio.NewReader(&stdout)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		module := Module{}
		buf := bytes.NewBufferString(line)
		if err := json.NewDecoder(buf).Decode(&module); err != nil {
			panic(err)
		}
		modules = append(modules, module)
	}

	var indirect []string
	for _, v := range modules {
		if v.Indirect {
			indirect = append(indirect, v.Name)
		}
	}

	return indirect
}
