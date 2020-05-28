package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var usagesCache map[string]map[string]int = make(map[string]map[string]int)

// ModuleUsagesForModule finds the number of times each of the given module's
// module dependencies are referred to.
//
// This is a thin cache wrapper around the real thing.
// TODO(deklerk): Maybe stick a more formal cache in main?
// TODO(deklerk): This is nice (and easy!), it's JIT caching. We shoud
// pre-populate the cache from main before even rendering.
func ModuleUsagesForModule(from, to string) int {
	if _, ok := usagesCache[from]; !ok {
		usagesCache[from] = make(map[string]int)
		numUsages := moduleUsagesForModule(from, to)
		usagesCache[from][to] = numUsages
		return numUsages
	}
	if _, ok := usagesCache[from][to]; !ok {
		numUsages := moduleUsagesForModule(from, to)
		usagesCache[from][to] = numUsages
		return numUsages
	}
	return usagesCache[from][to]
}

func moduleUsagesForModule(from, to string) int {
	fmt.Printf("ModuleUsagesForModule(%s, %s)\n", from, to)

	// TODO(deklerk): Capital letters need to be replaced with !lowercase. Ex
	// github.com/Shopify/sarama@v1.23.1 becomes github.com/!shopify/sarama@v1.23.1
	moduleRootPath := attemptToFindModuleOnFS(from)
	if moduleRootPath == "" {
		// TODO: We should probably try "go get" here.
		panic(fmt.Errorf("could not find module %s on file system. try `go get %s`?", from, from))
	}

	packageCounts := PackageUsagesForModule(moduleRootPath)

	for p, c := range packageCounts {
		// TODO(deklerk): This basically looks for the first module that looks
		// like the given package. This falls down in two places:
		// 1. v1 would match a v2 (substring match!). etc for all version
		// 2. github.com/user/module/foo/submod/pkg would match
		// github.com/user/module/foo/submod (good!) as well as
		// parent github.com/user/module (bad!)
		if strings.Contains(to, p) {
			return c
		}
	}

	return 0
}

// PackageUsagesForModule finds the number of times each of the given module's
// package dependencies are referred to.
func PackageUsagesForModule(moduleRootPath string) map[string]int {
	files := moduleFiles(moduleRootPath)

	moduleUsages := make(map[string]int)

	for _, f := range files {
		outBytes, err := ioutil.ReadFile(f)
		if err != nil {
			panic(err)
		}

		for k, v := range PackageUsages(string(outBytes)) {
			moduleUsages[k] += v
		}
	}

	return moduleUsages
}

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

// PackageUsages analyzes the given code, records the imported package, and
// counts the number of times that each imported package is used.
//
// It panics if it encounters a problem. (for better stacktraces heh)
func PackageUsages(src string) map[string]int {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "does-not-seem-to-matter.go", src, 0)
	if err != nil {
		panic(err)
	}

	out := map[string]int{}
	importNames := map[string]string{}
	importUsages := map[string]int{}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			if x.Name == nil {
				// Regular import.
				s := strings.Split(x.Path.Value, "/")
				name := s[len(s)-1]
				name = strings.Replace(name, "\"", "", -1)
				importNames[name] = strings.Replace(x.Path.Value, "\"", "", -1)
			} else {
				if x.Name.Name == "_" {
					// Underscore import.
					out[strings.Replace(x.Path.Value, "\"", "", -1)] = -1
				} else {
					// Named import.
					importNames[x.Name.Name] = strings.Replace(x.Path.Value, "\"", "", -1)
					// We count from -1 because the AST walk is going to find 1 extra
					// ident occurrence - the import name itself - which we don't want to
					// include.
					//
					// TODO(deklerk): This would be a bit easier to understand if we could
					// figure out during the ast.Inspect that the ident is part of an
					// import statement, and then not count it. But, a brief examination
					// didn't turn up anything fruitful, so here we are.
					importUsages[x.Name.Name] = -1
				}
			}
		}
		return true
	})

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if _, ok := importNames[x.Name]; ok {
				importUsages[x.Name]++
			}
		}
		return true
	})

	for shortName, usages := range importUsages {
		longName := importNames[shortName]
		out[longName] = usages
	}

	return out
}

func attemptToFindModuleOnFS(module string) string {
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
	err := cmd.Run()
	if err != nil {
		panic(fmt.Errorf("failed to run `cd %s && go list -json ./...`:\n%s\n%v", dir, stderr.String(), err))
	}

	if err := json.NewDecoder(&stdout).Decode(&decoded); err != nil {
		panic(err)
	}

	return decoded.Mod.Path, decoded.Mod.Dir
}

var uppperCaseRegex = regexp.MustCompile("[A-Z]")

// For some reason, all modules are stored on filesystem with capital letters
// replaced with a bang and the lower case. For example,
// github.com/Shopify/sarama@v1.23.1 becomes github.com/!shopify/sarama@v1.23.1.
func replaceCapitalLetters(s string) string {
	// TODO: This is silly but works. Maybe see if there's a better way.
	for _, upper := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"} {
		lower := strings.ToLower(upper)
		s = strings.ReplaceAll(s, upper, "!"+lower)
	}
	return s
}
