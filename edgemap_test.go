package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNegativeComplement(t *testing.T) {
	for _, tc := range []struct {
		desc string
		a    edgeMap
		b    edgeMap
		want edgeMap
	}{
		{
			desc: "empty one side, populated other side",
			a:    edgeMap{},
			b:    edgeMap{"a": makeEdge("a", "b")},
			want: edgeMap{"a": makeEdge("a", "b")},
		},
		{
			desc: "same on both side",
			a:    edgeMap{"a": makeEdge("a", "b")},
			b:    edgeMap{"a": makeEdge("a", "b")},
			want: edgeMap{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := tc.a.negativeComplement(tc.b)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Error("got different edgeMap (extraneous -, missing +): ", diff)
			}
		})
	}
}
