package main

import "testing"

func TestModulePath(t *testing.T) {
	oldGopathDir := gopathDir
	gopathDir = "/"
	defer func() {
		gopathDir = oldGopathDir
	}()

	oldFolderExists := folderExists
	folderExists = func(string) bool { return true }
	defer func() {
		folderExists = oldFolderExists
	}()

	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "github.com/user/lib@v1.2.3", want: "/pkg/mod/github.com/user/lib@v1.2.3"},
		// BurntSushi weirdly gets replaced with !burnt!sushi. https://github.com/golang/go/issues/36620
		{in: "github.com/BurntSushi/gfwl@v1.2.3", want: "/pkg/mod/github.com/!burnt!sushi/gfwl@v1.2.3"},
		// BurntSushi weirdly gets replaced with !burnt!sushi. https://github.com/golang/go/issues/36620
		{in: "github.com/user/lib/v3@v3.4.5", want: "/pkg/mod/github.com/user/lib@v3.4.5"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := modulePath(tc.in)
			if !ok {
				t.Fatal("expected to get a path, but got none")
			}
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
