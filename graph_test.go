package main

import (
	"bytes"
	"testing"
)

func TestRemoveVertex(t *testing.T) {
	for _, tc := range []struct {
		name   string
		in     string
		remove string
		want   string
	}{
		{name: "Basic", in: "A B", remove: "B", want: "A"},
		{name: "Prune one", in: "A B\nB C", remove: "B", want: "A"},
		{name: "Prune several", in: "A B\nB C\nB D", remove: "B", want: "A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotg, err := newGraph(bytes.NewBufferString(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if err := gotg.removeVertex(gotg.vertices[tc.remove]); err != nil {
				t.Fatal(err)
			}
			wantg, err := newGraph(bytes.NewBufferString(tc.want))
			if err != nil {
				t.Fatal(err)
			}
			if !gotg.Equal(wantg) {
				t.Fatalf("got %s, want %s", gotg, wantg)
			}
		})
	}
}

func TestHypotheticalCutVertex(t *testing.T) {
	for _, tc := range []struct {
		name       string
		in         string
		remove     string
		wantPruned []string
	}{
		{name: "No prune", in: "A B", remove: "B", wantPruned: []string{}},
		{name: "Prune one", in: "A B\nB C", remove: "B", wantPruned: []string{"C"}},
		{name: "Prune several", in: "A B\nB C\nB D", remove: "B", wantPruned: []string{"C", "D"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := newGraph(bytes.NewBufferString(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			gotPruned, err := g.hypotheticalCutVertex(g.vertices[tc.remove])
			if err != nil {
				t.Fatal(err)
			}

			if len(gotPruned) != len(tc.wantPruned) {
				t.Fatalf("want %v pruned, got %v pruned", tc.wantPruned, gotPruned)
			}
			gotPrunedMap := make(map[string]struct{})
			for _, p := range gotPruned {
				gotPrunedMap[p.Label] = struct{}{}
			}
			for _, v := range tc.wantPruned {
				if _, ok := gotPrunedMap[v]; !ok {
					t.Fatalf("want %v pruned, got %v pruned", tc.wantPruned, gotPruned)
				}
			}
		})
	}
}

func TestBuildVertexDominatorTree(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want *treeNode
	}{
		{
			name: "One child",
			in:   "A B",
			want: &treeNode{v: &Vertex{Label: "A"}, children: []*treeNode{
				{v: &Vertex{Label: "B"}},
			}},
		},
		{
			name: "Two children",
			in:   "A B\nA C",
			want: &treeNode{v: &Vertex{Label: "A"}, children: []*treeNode{
				{v: &Vertex{Label: "B"}},
				{v: &Vertex{Label: "C"}},
			}},
		},
		{
			name: "Chain",
			in:   "A B\nB C",
			want: &treeNode{v: &Vertex{Label: "A"}, children: []*treeNode{
				{v: &Vertex{Label: "B"}, children: []*treeNode{
					{v: &Vertex{Label: "C"}},
				}},
			}},
		},
		{
			name: "Chain two",
			in:   "A B\nB C\nB D",
			want: &treeNode{v: &Vertex{Label: "A"}, children: []*treeNode{
				{v: &Vertex{Label: "B"}, children: []*treeNode{
					{v: &Vertex{Label: "C"}},
					{v: &Vertex{Label: "D"}},
				}},
			}},
		},
		{
			name: "Recursion",
			in:   "A B\nB A",
			want: &treeNode{v: &Vertex{Label: "A"}, children: []*treeNode{
				{v: &Vertex{Label: "B"}},
			}},
		},
		{
			name: "Dominate",
			in:   "A B\nA C\nB C",
			want: &treeNode{v: &Vertex{Label: "A"}, children: []*treeNode{
				{v: &Vertex{Label: "B"}},
				{v: &Vertex{Label: "C"}},
			}},
		},
		{
			name: "Dominate one out",
			in:   "A B\nB C\nB D\nC D",
			want: &treeNode{v: &Vertex{Label: "A"}, children: []*treeNode{
				{v: &Vertex{Label: "B"}, children: []*treeNode{
					{v: &Vertex{Label: "C"}},
					{v: &Vertex{Label: "D"}},
				}},
			}},
		},
		// https://www.cs.au.dk/~gerth/advising/thesis/henrik-knakkegaard-christensen.pdf
		{
			name: "Complex",
			in:   "C D\nC G\nD H\nD A\nA B\nA F\nG E\nG I\nH J\nJ I\nJ K\nI E\nK C",
			want: &treeNode{v: &Vertex{Label: "C"}, children: []*treeNode{
				{v: &Vertex{Label: "D"}, children: []*treeNode{
					{v: &Vertex{Label: "H"}},
					{v: &Vertex{Label: "A"}, children: []*treeNode{
						{v: &Vertex{Label: "B"}},
						{v: &Vertex{Label: "F"}},
					}},
				}},
				{v: &Vertex{Label: "G"}, children: []*treeNode{
					{v: &Vertex{Label: "E"}},
					{v: &Vertex{Label: "I"}},
				}},
				{v: &Vertex{Label: "J"}, children: []*treeNode{
					{v: &Vertex{Label: "K"}},
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := bytes.NewBufferString(tc.in)
			g, err := newGraph(b)
			if err != nil {
				t.Fatal(err)
			}
			got, err := g.buildVertexDominatorTree()
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
