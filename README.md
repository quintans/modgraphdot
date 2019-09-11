# modgraphdot
Modgraphdot converts “go mod graph” output into Graphviz's DOT language

> This is a fork of https://github.com/golang/exp/tree/master/cmd/modgraphviz

Modgraphdot converts “go mod graph” output into Graphviz's DOT language,
for use with Graphviz visualization and analysis tools like dot, dotty, and sccmap.

Usage:

`go mod graph | modgraphdot > graph.dot`

`go mod graph | modgraphdot [stop string] | dot -Tpng -o graph.png`

Example:

`go mod graph | modgraphdot "go-grpc@" | dot -Tsvg -o graph.svg`

Install:

`go get github.com/quintans/modgraphdot`