package main

import (
	"fmt"

	"github.com/jadekler/lean/internal"
)

func main() {
	p := &internal.ASTParser{}
	// github.com/getlantern/idletiming -> github.com/aristanetworks/goarista@v0.0.0-20200131140622-c6473e3ed183
	from := "github.com/getlantern/idletiming"
	to := "github.com/aristanetworks/goarista@v0.0.0-20200131140622-c6473e3ed183"
	c := p.ModuleUsagesForModule(from, to)
	fmt.Println(c)
}
