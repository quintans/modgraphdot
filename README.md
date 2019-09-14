# modgraphdot
Modgraphdot converts “go mod graph” output into Graphviz's DOT language

> This is a fork of https://github.com/golang/exp/tree/master/cmd/modgraphviz

Modgraphdot converts “go mod graph” output into Graphviz's DOT language,
for use with Graphviz visualization and analysis tools like dot, dotty, and sccmap.

This tools allows us to create a tree of dependencies that stop at the dependency that we are looking for,
resulting in a shorter tree, if the `stop string` is defined]

For each module, the node representing the greatest version (i.e., the
version chosen by Go's minimal version selection algorithm) is colored green.
Other nodes, which aren't in the final build list, are colored grey.
If `-p` is set, only the the green ones will apper.

Usage:

`go mod graph | modgraphdot > graph.dot`

`go mod graph | modgraphdot [-p] [stop string] | dot -Tpng -o graph.png`

Example:

`go mod graph | modgraphdot "go-grpc@" | dot -Tsvg -o graph.svg`

will show ALL the paths from the main module to all module whose name contains "go-grps@" in a svg file.

`go mod graph | modgraphdot | dot -Tsvg -o graph.svg`

will show the full graph

`go mod graph | modgraphdot | dot -Tsvg -o graph.svg`

Will show only the paths use in the build

Install:

`go get -u github.com/quintans/modgraphdot`
