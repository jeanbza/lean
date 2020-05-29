package internal

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPackageUsages(t *testing.T) {
	for _, tc := range []struct {
		desc string
		src  string
		want map[string]int
	}{
		{
			desc: "Basic",
			src: `package main
import "github.com/foo/bar"
var X = bar.Y`,
			want: map[string]int{"github.com/foo/bar": 1},
		},
		{
			desc: "Multiple imports same package",
			src: `package main
import "github.com/foo/bar"
var X = bar.Y
var Z = bar.Y
var Q = bar.S`,
			want: map[string]int{"github.com/foo/bar": 3},
		},
		{
			desc: "Imports of different package",
			src: `package main
import "github.com/foo/bar"
import "github.com/foo/gaz"
var X = bar.Y
var Z = gaz.Y
var Q = bar.S`,
			want: map[string]int{"github.com/foo/bar": 2, "github.com/foo/gaz": 1},
		},
		{
			desc: "Non-github",
			src: `package main
import "google.golang.org/foo/bar"
var X = bar.Y`,
			want: map[string]int{"google.golang.org/foo/bar": 1},
		},
		{
			desc: "underscore import is -1",
			src: `package main
import _ "github.com/foo/bar"
var X = Y`,
			want: map[string]int{"github.com/foo/bar": -1},
		},
		{
			desc: "named import",
			src: `package main
import gaz "github.com/foo/bar"
var X = gaz.Y`,
			want: map[string]int{"github.com/foo/bar": 1},
		},
		{
			desc: "stdlib",
			src: `package main
import "os"
import "os/exec"
var X = os.ErrInvalid
var Y = os.ErrExist
var Z = exec.Cmd{}`,
			want: map[string]int{"os": 2, "os/exec": 1},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := PackageUsages(tc.src)

			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Fatalf("expected %v, got %v\n\t%s", tc.want, got, diff)
			}
		})
	}
}
