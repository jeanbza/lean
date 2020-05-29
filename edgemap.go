package main

import (
	"fmt"
	"sync"
)

type edge struct {
	From *Vertex
	To   *Vertex
	// Number of times that from uses to. (AST parsing)
	NumUsages int
}

func (e *edge) String() string {
	return fmt.Sprintf("{From: %s, To: %s, NumUsages: %d}", e.From, e.To, e.NumUsages)
}

// emMu protects edgeMap.
var emMu = sync.Mutex{}

// edgeMap is the map of edges in a graph.
type edgeMap map[string]map[string]*edge

// containsLocked returns whether there's a from-to edge in edgeMap.
//
// em must be locked.
func (em edgeMap) containsLocked(from, to *Vertex) bool {
	if em == nil {
		em = make(edgeMap)
	}
	if _, ok := em[from.Label]; !ok {
		return false
	}
	if _, ok := em[from.Label][to.Label]; !ok {
		return false
	}
	return true
}

// set creates an edge from-to.
func (em edgeMap) set(from, to *Vertex) {
	// This can take O(seconds), so let's do it outside the lock.
	numUsages := astParser.ModuleUsagesForModule(from.Label, to.Label)

	emMu.Lock()
	defer emMu.Unlock()
	if em == nil {
		em = make(edgeMap)
	}
	if _, ok := em[from.Label]; !ok {
		em[from.Label] = make(map[string]*edge)
	}
	em[from.Label][to.Label] = &edge{From: from, To: to, NumUsages: numUsages}
}

// remove removes the edge from-to.
func (em edgeMap) remove(from, to *Vertex) error {
	emMu.Lock()
	defer emMu.Unlock()

	if em == nil {
		em = make(edgeMap)
	}
	if !em.containsLocked(from, to) {
		return fmt.Errorf("edge (%s, %s) not found", from.Label, to.Label)
	}
	delete(em[from.Label], to.Label)
	return nil
}

// vertices returns all vertices.
func (em edgeMap) vertices() []*Vertex {
	emMu.Lock()
	defer emMu.Unlock()

	var out []*Vertex
	for _, vs := range em {
		for _, v := range vs {
			out = append(out, v.From)
			out = append(out, v.To)
		}
	}
	return out
}

// Returns !(a \ b).
// https://en.wikipedia.org/wiki/Complement_(set_theory)
func (a edgeMap) negativeComplement(b edgeMap) edgeMap {
	emMu.Lock()
	defer emMu.Unlock()

	out := make(edgeMap)
	for bFrom, bTos := range b {
		for bTo, bEdge := range bTos {
			if _, ok := a[bFrom]; ok {
				if _, ok := a[bFrom][bTo]; ok {
					continue
				}
			}
			if _, ok := out[bFrom]; !ok {
				out[bFrom] = make(map[string]*edge)
			}
			out[bFrom][bTo] = bEdge
		}
	}
	return out
}

func (em edgeMap) String() string {
	s := "{"
	for from, vs := range em {
		if len(vs) == 0 {
			s += fmt.Sprintf("\n\t%s: {},", from)
			continue
		}

		s += fmt.Sprintf("\n\t%s: {", from)
		for to, v := range vs {
			s += fmt.Sprintf("\n\t\t%s: %s,", to, v)
		}
		s += "\n\t},"
	}
	s += "\n}"
	return s
}
