package main

import (
	"fmt"

	"github.com/jadekler/lean/internal"
)

func main() {
	p := &internal.ASTParser{}
	c := p.ModuleUsagesForModule("golang.org/x/net@v0.0.0-20190311183353-d8887717615a", "golang.org/x/text@v0.3.0")
	fmt.Println(c)
}
