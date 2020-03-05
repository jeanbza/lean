package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jadekler/lean/internal"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) != 2 {
			panic(fmt.Errorf("expected 2 words in line, but got %d: %s", len(parts), l))
		}
		from := parts[0]
		to := parts[1]

		usagesFrom, err := internal.ModuleUsagesForModule(from)
		if err != nil {
			panic(err)
		}
		fmt.Println(from, usagesFrom)

		usagesTo, err := internal.ModuleUsagesForModule(to)
		if err != nil {
			panic(err)
		}
		fmt.Println(to, usagesTo)
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}
