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
	"strings"
)

// PackageUsagesForModule finds the number of times each of the given module's
// module dependencies are referred to.
func ModuleUsagesForModule(module string) map[string]int {
	moduleRootPath := attemptToFindModuleOnFS(module)
	if moduleRootPath == "" {
		panic(fmt.Errorf("could not find module %s on file system", module))
	}

	packageCounts := PackageUsagesForModule(moduleRootPath)

	moduleCounts := make(map[string]int)
	for p, c := range packageCounts {
		mn := moduleName(p)
		moduleCounts[mn] += c
	}

	return moduleCounts
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

func moduleName(packageName string) string {
	parts := strings.Split(packageName, "/")

	for i := range parts {
		modulePathParts := append([]string{build.Default.GOPATH, "pkg", "mod"}, parts[0:len(parts)-1-i]...)
		fi, err := os.Stat(filepath.Join(modulePathParts...))
		if err == os.ErrNotExist {
			continue
		}
		if err != nil {
			continue
		}
		if fi.IsDir() {
			return strings.Join(parts[0:len(parts)-1-i], "/")
		}
	}

	panic(errors.New("this should not have happened..."))
}

// moduleFiles finds all the files of a module (given the module's root path).
func moduleFiles(moduleRootPath string) []string {
	type GoList struct {
		Dir         string   `json:"Dir"`
		GoFiles     []string `json:"GoFiles"`
		TestGoFiles []string `json:"XTestGoFiles"`
	}

	cmd := exec.Command("go", "list", "-json", "./...")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Dir = moduleRootPath
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	var files []string
	dec := json.NewDecoder(&out)
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
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	if err := json.NewDecoder(&out).Decode(&decoded); err != nil {
		panic(err)
	}

	return decoded.Mod.Path, decoded.Mod.Dir
}
