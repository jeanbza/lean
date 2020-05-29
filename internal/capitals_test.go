package internal

import "testing"

func TestReplaceCapitalLetters(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{
			in:   "",
			want: "",
		},
		{
			in:   "foo",
			want: "foo",
		},
		{
			in:   "FoO",
			want: "!fo!o",
		},
		{
			in:   "github.com/Shopify/sarama@v1.23.1",
			want: "github.com/!shopify/sarama@v1.23.1",
		},
	} {
		t.Run(tc.in, func(t *testing.T) {
			if got, want := replaceCapitalLetters(tc.in), tc.want; got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}
}
