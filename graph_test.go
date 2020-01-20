package main

import "testing"

import "bytes"

func TestBuildDominatorTree(t *testing.T) {
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
			got := g.buildDominatorTree()
			if !got.Equal(tc.want) {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
