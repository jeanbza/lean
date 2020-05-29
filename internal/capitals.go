package internal

import (
	"strings"
)

// replaceCapitalLetters replaces capital letters with bang lowercase. For
// example, S becomes !s.
//
// For some reason, all modules are stored on filesystem with capital letters
// replaced with a bang and the lower case. For example,
// github.com/Shopify/sarama@v1.23.1 becomes github.com/!shopify/sarama@v1.23.1.
func replaceCapitalLetters(s string) string {
	// TODO: This is silly but works. Maybe see if there's a better way.
	for _, upper := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"} {
		lower := strings.ToLower(upper)
		s = strings.ReplaceAll(s, upper, "!"+lower)
	}
	return s
}
