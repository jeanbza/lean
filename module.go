// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func moduleSize(module string) (int64, error) {
	path, ok := modulePath(module)
	if !ok {
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

// modulePath returns the path on the filesystem of the module. If it can't find
// the path, it return false.
//
// TODO: Figure out how to get at /Users/deklerk/go/pkg/mod/github.com/go-gl/glfw\@v0.0.0-20190409004039-e6da0acd62b1/v3.2/glfw/.
func modulePath(module string) (string, bool) {
	out := strings.Split(module, "@")
	if len(out) == 1 { // There's no @ in the module - it's the root module.
		return "", false
	}

	moduleName := out[0]
	// Throw out the /v3 at end of module name.
	if versionRegexp.MatchString(moduleName) {
		moduleName = versionRegexp.FindAllStringSubmatch(moduleName, -1)[0][1]
	}
	moduleNameParts := strings.Split(moduleName, "/")

	nameFirstParts := moduleNameParts[:len(moduleNameParts)-1]

	// Hack: some modules are stored with different names for some inexplicable
	// reason. https://github.com/golang/go/issues/36620
	for i, v := range nameFirstParts {
		if v == "BurntSushi" {
			nameFirstParts[i] = "!burnt!sushi"
		}
	}

	nameLastPart := moduleNameParts[len(moduleNameParts)-1]

	moduleVersion := out[1]

	modulePathParts := append([]string{build.Default.GOPATH, "pkg", "mod"}, nameFirstParts...)
	modulePathParts = append(modulePathParts, nameLastPart+"@"+moduleVersion)
	modulePath := filepath.Join(modulePathParts...)

	if _, err := os.Stat(modulePath); os.IsNotExist(err) {
		return "", false
	}

	return modulePath, true
}
