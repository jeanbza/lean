package main

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Implements ReplaceableASTParser.
type testASTParser struct{}

func (*testASTParser) ModuleUsagesForModule(string, string) int {
	return 0
}

func TestHypotheticalCut(t *testing.T) {
	astParser = &testASTParser{}

	for _, tc := range []struct {
		desc         string
		in           string
		cutFrom      string
		cutTo        string
		wantEdges    edgeMap  // removed edges
		wantVertices []string // removed vertices
	}{
		{
			desc:         "basic cut",
			in:           "github.com/foo github.com/bar",
			cutFrom:      "github.com/foo",
			cutTo:        "github.com/bar",
			wantEdges:    edgeMap{"github.com/foo": makeEdge("github.com/foo", "github.com/bar")},
			wantVertices: []string{"github.com/bar"},
		},
		{
			desc: "two cut",
			in: `github.com/foo github.com/bar
github.com/bar github.com/gaz`,
			cutFrom: "github.com/foo",
			cutTo:   "github.com/bar",
			wantEdges: edgeMap{
				"github.com/foo": makeEdge("github.com/foo", "github.com/bar"),
				"github.com/bar": makeEdge("github.com/bar", "github.com/gaz"),
			},
			wantVertices: []string{"github.com/bar", "github.com/gaz"},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			b := bytes.Buffer{}
			if _, err := b.WriteString(tc.in); err != nil {
				t.Fatal(err)
			}
			g, err := newGraph(&b)
			if err != nil {
				t.Fatal(err)
			}
			gotEdges, gotVertices, err := g.hypotheticalCut(tc.cutFrom, tc.cutTo)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(gotEdges, tc.wantEdges); diff != "" {
				t.Errorf("got different edges (extraneous -, missing +):\n%s", diff)
			}
			if diff := cmp.Diff(gotVertices, tc.wantVertices); diff != "" {
				t.Errorf("got different vertices (extraneous -, missing +):\n%s", diff)
			}
		})
	}
}

func TestConnected(t *testing.T) {
	astParser = &testASTParser{}

	for _, tc := range []struct {
		desc string
		root string
		in   string
		want edgeMap
	}{
		{
			desc: "basic",
			root: "a",
			in:   "a b",
			want: edgeMap{"a": makeEdge("a", "b")},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			buf := bytes.Buffer{}
			if _, err := buf.WriteString(tc.in); err != nil {
				t.Fatal(err)
			}
			g, err := newGraph(&buf)
			if err != nil {
				t.Fatal(err)
			}
			got := g.connected(tc.root)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("got different edges (extraneous -, missing +):\n%s", diff)
			}
		})
	}
}

func makeEdge(from, to string) map[string]*edge {
	return map[string]*edge{
		to: {
			From: &Vertex{Label: from, SizeBytes: -1},
			To:   &Vertex{Label: to, SizeBytes: -1},
		},
	}
}
