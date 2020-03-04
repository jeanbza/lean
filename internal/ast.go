package internal

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os/exec"
	"strings"
)

// PackageUsagesForModule finds the number of times each of the given module's
// module dependencies are referred to.
func ModuleUsagesForModule(moduleRootPath string) (map[string]int, error) {
	return nil, nil
}

// PackageUsagesForModule finds the number of times each of the given module's
// package dependencies are referred to.
func PackageUsagesForModule(moduleRootPath string) (map[string]int, error) {
	files, err := moduleFiles(moduleRootPath)
	if err != nil {
		return nil, err
	}

	moduleUsages := make(map[string]int)

	for _, f := range files {
		outBytes, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}

		usages, err := PackageUsages(string(outBytes))
		if err != nil {
			return nil, err
		}

		for k, v := range usages {
			moduleUsages[k] += v
		}
	}

	return moduleUsages, nil
}

type goListItem struct {
	GoFiles []string `json:"GoFiles"`
	TestGoFiles []string `json:"XTestGoFiles"`
}

// moduleFiles finds all the files of a module (given the module's root path).
func moduleFiles(moduleRootPath string) ([]string, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Dir = moduleRootPath
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	var outDecoded []goListItem
	if err := json.NewDecoder(&out).Decode(&outDecoded); err != nil {
		return nil, err
	}

	var files []string
	for _, i := range outDecoded {
		files = append(files, i.GoFiles...)
		files = append(files, i.TestGoFiles...)
	}

	return files, nil
}

// PackageUsages analyzes the given code, records the imported package, and
// counts the number of times that each imported package is used.
func PackageUsages(src string) (map[string]int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "does-not-seem-to-matter.go", src, 0)
	if err != nil {
		return nil, err
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

	return out, nil
}
