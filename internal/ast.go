package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"strings"
	"sync"
)

type ASTParser struct{}

// cacheMu protects usagesCache.
var cacheMu = sync.Mutex{}

// usagesCache is a cache of ModuleUsagesForModule. It keeps track of the
// number of references to "to" module in each "from" module.
//
// It's cached in ast because the number of references is not expected to change
// anywhere in the running of this program.
var usagesCache map[string]map[string]int = make(map[string]map[string]int)

// ModuleUsagesForModule finds the number of times each of the given module's
// module dependencies are referred to.
//
// This is a thin cache wrapper around the real thing.
func (*ASTParser) ModuleUsagesForModule(from, to string) int {
	cacheMu.Lock()
	if v, ok := usagesCache[from][to]; ok {
		cacheMu.Unlock()
		return v
	}
	cacheMu.Unlock()
	numUsages := moduleUsagesForModule(from, to)

	cacheMu.Lock()
	defer cacheMu.Unlock()
	if _, ok := usagesCache[from]; !ok {
		usagesCache[from] = make(map[string]int)
	}
	if _, ok := usagesCache[from][to]; !ok {
		usagesCache[from][to] = numUsages
	}
	return usagesCache[from][to]
}

func moduleUsagesForModule(from, to string) int {
	fmt.Printf("Analyzing edge (%s, %s)\n", from, to)

	moduleRootPath := attemptToFindModuleOnFS(from)
	if moduleRootPath == "" {
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
