// this is a fork from https://github.com/golang/exp/tree/master/cmd/modgraphviz

// Modgraphdot converts “go mod graph” output into Graphviz's DOT language,
// for use with Graphviz visualization and analysis tools like dot, dotty, and sccmap.
//
// Usage:
//
//	go mod graph | modgraphdot > graph.dot
//	go mod graph | modgraphdot [stop string] | dot -Tpng -o graph.png
//
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: go mod graph | modgraphdot [-p] [stop string] | dot -Tpng -o graph.png

For each module, the node representing the greatest version (i.e., the
version chosen by Go's minimal version selection algorithm) is colored green.
Other nodes, which aren't in the final build list, are colored grey.
If -p is set, only the the green ones will apper.
If [stop string] is defined 
`)
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("modgraphdot: ")

	flag.Usage = usage
	var onlyPicked = flag.Bool("p", false, "if set, only the picked versions are displayed")
	flag.Parse()
	if flag.NArg() > 1 {
		usage()
	}

	stopAt := flag.Arg(0)

	if err := modgraphdot(os.Stdin, os.Stdout, *onlyPicked, stopAt); err != nil {
		log.Fatal(err)
	}
}

func modgraphdot(in io.Reader, out io.Writer, onlyPicked bool, stopAt string) error {
	graph, err := convert(in)
	if err != nil {
		return err
	}
	if stopAt != "" {
		graph.trim(onlyPicked, stopAt)
	}

	fmt.Fprintf(out, "digraph gomodgraph {\n")
	out.Write(graph.edgesAsDOT())
	for _, n := range graph.mvsPicked {
		fmt.Fprintf(out, "\t%q [style = filled, fillcolor = green]\n", n)
	}
	for _, n := range graph.mvsUnpicked {
		fmt.Fprintf(out, "\t%q [style = filled, fillcolor = gray]\n", n)
	}
	fmt.Fprintf(out, "}\n")

	return nil
}

type edge struct{ from, to string }
type graph struct {
	edges       []edge
	mvsPicked   []string
	mvsUnpicked []string
}

func (g *graph) trim(onlyPicked bool, stopAt string) {
	mvsPicked := map[string]bool{}
	if onlyPicked {
		for _, v := range g.mvsPicked {
			mvsPicked[v] = true
		}
	}
	nodes := make(map[string]*node)
	// from
	for _, v := range g.edges {
		addToNodes(nodes, v.from, mvsPicked)
	}
	// to
	for _, v := range g.edges {
		addToNodes(nodes, v.to, mvsPicked)
	}
	// relate from -> to
	for _, v := range g.edges {
		from := nodes[v.from]
		to := nodes[v.to]
		from.nodes = append(from.nodes, to)
	}

	root := findRoot(nodes)

	seen := map[string]bool{}
	edges := make([]edge, 0)
	edges, _ = root.toEdges(seen, edges, onlyPicked, stopAt)
	g.edges = removeDuplicateEdges(edges)

	currentEdges := map[string]bool{}
	for _, v := range edges {
		currentEdges[v.from] = true
		currentEdges[v.to] = true
	}

	g.mvsPicked = filterOut(currentEdges, g.mvsPicked)
	g.mvsUnpicked = filterOut(currentEdges, g.mvsUnpicked)
}

type node struct {
	name   string
	nodes  []*node
	picked bool
}

func newNode(name string, picked bool) *node {
	return &node{name, make([]*node, 0), picked}
}

func (n *node) toEdges(seen map[string]bool, edges []edge, onlyPicked bool, stopAt string) ([]edge, bool) {
	if seen[n.name] {
		return edges, false
	}

	if strings.Contains(n.name, stopAt) {
		return edges, !onlyPicked || n.picked
	}

	seen[n.name] = true
	found := false
	for _, v := range n.nodes {
		if onlyPicked && !v.picked {
			continue
		}

		edges = append(edges, edge{from: n.name, to: v.name}) // Push
		if children, ok := v.toEdges(seen, edges, onlyPicked, stopAt); ok {
			edges = children
			found = true
		} else {
			edges = edges[:len(edges)-1] // Pop
		}
	}
	delete(seen, n.name)

	return edges, found
}

func findRoot(nodes map[string]*node) *node {
	var root *node
	for k, v := range nodes {
		if strings.IndexByte(k, '@') == -1 {
			root = v
			root.picked = true
			return root
		}
	}
	panic("There is no root node!!!")
}

func removeDuplicateEdges(edges []edge) []edge {
	for i := len(edges) - 1; i >= 0; i-- {
		e := edges[i]
		// check equality
		for k, v := range edges {
			if k != i && e.from == v.from && e.to == v.to {
				// delete
				edges = append(edges[:i], edges[i+1:]...)
				break
			}
		}
	}
	return edges
}

func filterOut(existing map[string]bool, toFilter []string) []string {
	filtered := make([]string, 0)
	for _, v := range toFilter {
		if existing[v] {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// FIXME: side effect
func addToNodes(nodes map[string]*node, name string, msvPicked map[string]bool) {
	if _, ok := nodes[name]; !ok {
		picked := msvPicked[name]
		nodes[name] = newNode(name, picked)
	}
}

// convert reads “go mod graph” output from r and returns a graph, recording
// MVS picked and unpicked nodes along the way.
func convert(r io.Reader) (*graph, error) {
	scanner := bufio.NewScanner(r)
	var g graph
	seen := map[string]bool{}
	mvsPicked := map[string]string{} // module name -> module version

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
		g.edges = append(g.edges, edge{from: from, to: to})

		for _, node := range []string{from, to} {
			if _, ok := seen[node]; ok {
				// Skip over nodes we've already seen.
				continue
			}
			seen[node] = true

			var m, v string
			if i := strings.IndexByte(node, '@'); i >= 0 {
				m, v = node[:i], node[i+1:]
			} else {
				// Root node doesn't have a version.
				continue
			}

			if maxV, ok := mvsPicked[m]; ok {
				if semver.Compare(maxV, v) < 0 {
					// This version is higher - replace it and consign the old
					// max to the unpicked list.
					g.mvsUnpicked = append(g.mvsUnpicked, m+"@"+maxV)
					mvsPicked[m] = v
				} else {
					// Other version is higher - stick this version in the
					// unpicked list.
					g.mvsUnpicked = append(g.mvsUnpicked, node)
				}
			} else {
				mvsPicked[m] = v
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for m, v := range mvsPicked {
		g.mvsPicked = append(g.mvsPicked, m+"@"+v)
	}

	// Make this function deterministic.
	sort.Strings(g.mvsPicked)

	return &g, nil
}

// edgesAsDOT returns the edges in DOT notation.
func (g *graph) edgesAsDOT() []byte {
	var buf bytes.Buffer
	for _, e := range g.edges {
		fmt.Fprintf(&buf, "\t%q -> %q\n", e.from, e.to)
	}
	return buf.Bytes()
}
