package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
)

type Vertex struct {
	Label     string
	SizeBytes int64
}

type edge struct {
	From *Vertex
	To   *Vertex
	// Number of times that from uses to. (AST parsing)
	numUsages int64
}

// edgeMap is the map of edges in a graph.
type edgeMap map[string]map[string]*edge

func (em edgeMap) contains(from, to *Vertex) bool {
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

func (em edgeMap) set(from, to *Vertex) {
	if em == nil {
		em = make(edgeMap)
	}
	if _, ok := em[from.Label]; !ok {
		em[from.Label] = make(map[string]*edge)
	}
	em[from.Label][to.Label] = &edge{From: from, To: to}
}

func (em edgeMap) remove(from, to *Vertex) error {
	if em == nil {
		em = make(edgeMap)
	}
	if !em.contains(from, to) {
		return fmt.Errorf("edge (%s, %s) not found", from.Label, to.Label)
	}
	delete(em[from.Label], to.Label)
	return nil
}

func (em edgeMap) removeVertex(v *Vertex) {
	if em == nil {
		em = make(edgeMap)
	}
	for from, tos := range em {
		if from == v.Label {
			delete(em, from)
		}
		for to := range tos {
			if to == v.Label {
				delete(em[from], to)
			}
		}
	}
}

func (em edgeMap) vertices() []*Vertex {
	var out []*Vertex
	for _, vs := range em {
		for _, v := range vs {
			out = append(out, v.From)
			out = append(out, v.To)
		}
	}
	return out
}

func (em edgeMap) edges() []*edge {
	var out []*edge
	for _, es := range em {
		for _, e := range es {
			out = append(out, e)
		}
	}
	return out
}

// Returns !(a \ b).
// https://en.wikipedia.org/wiki/Complement_(set_theory)
func (a edgeMap) negativeComplement(b edgeMap) edgeMap {
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
	edges    edgeMap
}

func newGraph(r io.Reader) (*graph, error) {
	g := &graph{vertices: make(map[string]*Vertex), edges: edgeMap{}}
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

		g.edges.set(g.vertices[from], g.vertices[to])

		// `go mod graph` always presents the root as the first "from" node
		if g.root == "" {
			g.root = from
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return g, nil
}

// copy creates a copy of g.
func (g *graph) copy() *graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	newg := &graph{
		root:     g.root,
		vertices: make(map[string]*Vertex),
		edges:    edgeMap{},
	}
	for k, v := range g.vertices {
		newg.vertices[k] = v
	}

	for _, edges := range g.edges {
		for _, edge := range edges {
			newg.edges.set(edge.From, edge.To)
		}
	}

	return newg
}

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

func (g *graph) removeVertex(v *Vertex) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.vertices[v.Label]; !ok {
		return fmt.Errorf("vertex %s does not exist", v.Label)
	}
	delete(g.vertices, v.Label)
	g.edges.removeVertex(v)

	return nil
}

// connected returns the subgraph that is reachable from root.
func (g *graph) connected(root string) edgeMap {
	g.mu.Lock()
	defer g.mu.Unlock()

	sub := edgeMap{}
	seenVertices := make(map[string]struct{})
	var dfs func(from string)
	dfs = func(from string) {
		if _, ok := g.edges[from]; !ok {
			return
		}
		if _, ok := seenVertices[from]; ok {
			return
		}
		seenVertices[from] = struct{}{}
		for _, e := range g.edges[from] {
			sub.set(e.From, e.To)
			dfs(e.To.Label)
		}
	}
	dfs(root)
	return sub
}

// hypotheticalCutEdge returns a list of edges and vertices that would be pruned
// from the graph if the given (from, to) edge were cut. These lists may be
// empty but will not be nil.
//
// TODO(deklerk): This is inefficient and should be converted to Lengauer-Tarjan
// https://www.cs.au.dk/~gerth/advising/thesis/henrik-knakkegaard-christensen.pdf.
func (g *graph) hypotheticalCutEdge(from, to string) (edgeMap, []*Vertex, error) {
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

// hypotheticalCutVertex returns a list of vertices that would be pruned if
// v were cut.
func (g *graph) hypotheticalCutVertex(v *Vertex) ([]*Vertex, error) {
	g.mu.Lock()
	if _, ok := g.vertices[v.Label]; !ok {
		g.mu.Unlock()
		return nil, fmt.Errorf("vertex %s does not exist", v.Label)
	}

	var vChildren []*Vertex
	for _, c := range g.edges[v.Label] {
		vChildren = append(vChildren, c.To)
	}
	g.mu.Unlock()

	cut := g.copy()
	if err := cut.removeVertex(v); err != nil {
		return nil, err
	}

	a := cut.connected(cut.root)
	reachableFromChildren := make(map[*Vertex]struct{})
	for _, c := range vChildren {
		b := cut.connected(c.Label)
		for _, bv := range b.vertices() {
			reachableFromChildren[bv] = struct{}{}
		}
	}
	var b []*Vertex
	for bv := range reachableFromChildren {
		b = append(b, bv)
	}

	return negativeComplementVertices(a.vertices(), b), nil
}

// Returns !(a \ b).
// https://en.wikipedia.org/wiki/Complement_(set_theory)
func negativeComplementVertices(a, b []*Vertex) []*Vertex {
	amap := make(map[*Vertex]struct{})
	for _, v := range a {
		amap[v] = struct{}{}
	}

	out := []*Vertex{}
	for _, v := range b {
		if _, ok := amap[v]; ok {
			continue
		}
		out = append(out, v)
	}
	return out
}

type treeNode struct {
	v        *Vertex
	parent   *treeNode
	children []*treeNode
}

// TODO locking/unlocking
// TODO https://www.cs.au.dk/~gerth/advising/thesis/henrik-knakkegaard-christensen.pdf
func (g *graph) buildVertexDominatorTree() (root *treeNode, _ error) {
	dominates := make(map[*Vertex][]*Vertex)
	treeMap := make(map[*Vertex]*treeNode)

	for _, v := range g.vertices {
		cutVs, err := g.hypotheticalCutVertex(v)
		if err != nil {
			return nil, err
		}
		dominates[v] = cutVs
		treeMap[v] = &treeNode{}
	}

	for dominator, dominatees := range dominates {
		dominatorTreeNode := treeMap[dominator]

	dominateesLoop:
		for _, dv := range dominatees {
			dvTreeNode := treeMap[dv]
			if dvTreeNode.parent == nil {
				dominatorTreeNode.children = append(dominatorTreeNode.children, dvTreeNode)
				dvTreeNode.parent = dominatorTreeNode
			} else {
				in := func(haystack []*Vertex, needle *Vertex) bool {
					for _, v := range haystack {
						if needle == v {
							return true
						}
					}
					return false
				}

				for parent := dvTreeNode.parent; parent != nil; parent = parent.parent {
					if parent.parent == nil && in(dominatees, parent.v) {
						// w' is dominated by w. So, w' must be a child of w.
						dominatorTreeNode.children = append(dominatorTreeNode.children, parent)
						parent.parent = dominatorTreeNode
						continue dominateesLoop
					}

					w1prime := parent.parent
					w2prime := parent
					if in(dominatees, w2prime.v) && in(dominates[w1prime.v], dv) {
						// w should go in between w'1 and w'2 (all of which are
						// above v).
						if dvTreeNode.parent != nil {
							panic("ahhh! dvTreeNode already has a parent, though. wtf?")
						}

						dvTreeNode.parent = w2prime
						w2prime.children = append(w2prime.children, dvTreeNode)

						dvTreeNode.children = append(dvTreeNode.children, w1prime)
						w1prime.parent = dvTreeNode

						continue dominateesLoop
					}
				}

				panic("ahhh! couldn't find a place for this vertex!")
			}
		}
	}

	return treeMap[g.vertices[g.root]], nil
}

// Used for tests.
func (a *treeNode) Equal(b *treeNode) bool {
	if a == nil && b == nil {
		return true
	}

	if (a == nil && b != nil) || (a != nil && b == nil) {
		return false
	}

	if (a.v == nil && b.v != nil) || (a.v != nil && b.v == nil) {
		return false
	}

	if a.v == nil && b.v == nil {
		return true
	}

	if a.v.Label != b.v.Label {
		return false
	}

	acs := make(map[string]struct{})
	for _, c := range a.children {
		acs[c.v.Label] = struct{}{}
	}

	bcs := make(map[string]*treeNode)
	for _, c := range b.children {
		bcs[c.v.Label] = c
	}

	if len(acs) != len(bcs) {
		return false
	}

	for c := range acs {
		if _, ok := bcs[c]; !ok {
			return false
		}

		if !a.Equal(bcs[c]) {
			return false
		}
	}

	return true
}

// Used for tests.
func (tn *treeNode) String() string {
	if tn == nil {
		return "<nil>"
	}

	if tn.children == nil {
		return "{ }"
	}

	if len(tn.children) == 0 {
		return tn.v.Label
	}

	out := fmt.Sprintf("%s { ", tn.v.Label)
	for i, c := range tn.children {
		if i > 0 {
			out += " "
		}
		out += c.String()
	}
	out += " }"
	return out
}
