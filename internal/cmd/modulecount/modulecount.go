package main

import (
	"fmt"

	"github.com/jadekler/lean/internal"
)

func main() {
	p := &internal.ASTParser{}
	from := "github.com/aristanetworks/goarista@v0.0.0-20200131140622-c6473e3ed183"
	to := "github.com/stretchr/testify@v1.3.0"
	c := p.ModuleUsagesForModule(from, to)
	fmt.Println(c)
}
