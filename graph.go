package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/jadekler/lean/internal"
)

type Vertex struct {
	Label     string
	SizeBytes int64
	// All vertices recursively dominated by this vertex.
	// NOT YET IMPLEMENTED.
	dominatees []*Vertex
}

type edge struct {
	From *Vertex
	To   *Vertex
	// Number of times that from uses to. (AST parsing)
	NumUsages int
}

// emMu protects edgeMap.
var emMu = sync.Mutex{}

// edgeMap is the map of edges in a graph.
type edgeMap map[string]map[string]*edge

// contains returns whether there's a from-to edge in edgeMap.
func (em edgeMap) contains(from, to *Vertex) bool {
	emMu.Lock()
	defer emMu.Unlock()

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
	numUsages := internal.ModuleUsagesForModule(from.Label, to.Label)

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
	if !em.contains(from, to) {
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

type graph struct {
	root string

	mu       sync.Mutex
	vertices map[string]*Vertex
	edges    *edgeMap
}

// newGraph creates a new graph.
func newGraph(r io.Reader) (*graph, error) {
	emWorkers := make(chan bool, 20)
	emWg := sync.WaitGroup{}

	g := &graph{vertices: make(map[string]*Vertex), edges: &edgeMap{}}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) != 2 {
			return nil, fmt.Errorf("expected 2 words in line, but got %d: %s", len(parts), l)
		}
		from := parts[0]
		to := parts[1]

		if _, ok := g.vertices[from]; !ok {
			sizeBytes, err := moduleSize(from)
			if err != nil {
				return nil, err
			}
			g.vertices[from] = &Vertex{Label: from, SizeBytes: sizeBytes}
		}
		if _, ok := g.vertices[to]; !ok {
			sizeBytes, err := moduleSize(to)
			if err != nil {
				return nil, err
			}
			g.vertices[to] = &Vertex{Label: to, SizeBytes: sizeBytes}
		}

		// set performs ast calculations, which takes O(seconds). So, let's
		// do so in a goroutine.
		//
		// TODO(deklerk) Should this happen elsewhere, so that these goroutine
		// and mutex interactions are a bit more clear?
		emWorkers <- true
		emWg.Add(1)
		go func() {
			g.edges.set(g.vertices[from], g.vertices[to])
			emWg.Done()
			<-emWorkers
		}()

		// `go mod graph` always presents the root as the first "from" node
		if g.root == "" {
			g.root = from
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	emWg.Wait()
	return g, nil
}

// copy creates a copy of g.
func (g *graph) copy() *graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	newg := &graph{
		root:     g.root,
		vertices: make(map[string]*Vertex),
		edges:    &edgeMap{},
	}
	for k, v := range g.vertices {
		newg.vertices[k] = v
	}

	for _, edges := range *g.edges {
		for _, edge := range edges {
			newg.edges.set(edge.From, edge.To)
		}
	}

	return newg
}

// addEdge adds an edge to the graph.
func (g *graph) addEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.vertices[from]; !ok {
		return fmt.Errorf("vertex %s does not exist", from)
	}
	if _, ok := g.vertices[to]; !ok {
		return fmt.Errorf("vertex %s does not exist", to)
	}
	g.edges.set(g.vertices[from], g.vertices[to])
	return nil
}

// removeEdge removes an edge from the graph.
func (g *graph) removeEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.vertices[from]; !ok {
		return fmt.Errorf("vertex %s does not exist", from)
	}
	if _, ok := g.vertices[to]; !ok {
		return fmt.Errorf("vertex %s does not exist", to)
	}
	return g.edges.remove(g.vertices[from], g.vertices[to])
}

// connected returns the subgraph that is reachable from root.
func (g *graph) connected(root string) edgeMap {
	g.mu.Lock()
	defer g.mu.Unlock()

	sub := edgeMap{}
	seenVertices := make(map[string]struct{})
	var dfs func(from string)
	dfs = func(from string) {
		if _, ok := (*g.edges)[from]; !ok {
			return
		}
		if _, ok := seenVertices[from]; ok {
			return
		}
		seenVertices[from] = struct{}{}
		for _, e := range (*g.edges)[from] {
			sub.set(e.From, e.To)
			dfs(e.To.Label)
		}
	}
	dfs(root)
	return sub
}

// hypotheticalCut returns a list of edges and vertices that would be pruned
// from the graph if the given (from, to) edge were cut. These lists may be
// empty but will not be nil.
//
// TODO(deklerk): This is inefficient and should be converted to Lengauer-Tarjan
// https://www.cs.au.dk/~gerth/advising/thesis/henrik-knakkegaard-christensen.pdf.
func (g *graph) hypotheticalCut(from, to string) (edgeMap, []string, error) {
	g.mu.Lock()
	if _, ok := g.vertices[from]; !ok {
		g.mu.Unlock()
		return nil, nil, fmt.Errorf("vertex %s does not exist", from)
	}
	if _, ok := g.vertices[to]; !ok {
		g.mu.Unlock()
		return nil, nil, fmt.Errorf("vertex %s does not exist", to)
	}
	if !g.edges.contains(g.vertices[from], g.vertices[to]) {
		g.mu.Unlock()
		return nil, nil, fmt.Errorf("edge (%s, %s) does not exist", from, to)
	}
	g.mu.Unlock()

	cut := g.copy()
	if err := cut.removeEdge(from, to); err != nil {
		return nil, nil, err
	}

	a := cut.connected(cut.root)
	b := cut.connected(to)

	return a.negativeComplement(b), negativeComplementVertices(a.vertices(), b.vertices()), nil
}

// Returns !(a \ b).
// https://en.wikipedia.org/wiki/Complement_(set_theory)
func negativeComplementVertices(a, b []*Vertex) []string {
	amap := make(map[*Vertex]struct{})
	for _, v := range a {
		amap[v] = struct{}{}
	}

	out := []string{}
	for _, v := range b {
		if _, ok := amap[v]; ok {
			continue
		}
		out = append(out, v.Label)
	}
	return out
}
