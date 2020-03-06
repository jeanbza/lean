# countmoduleusages

countmoduleusages is a simple utility that, given a module graph, reports the
usages of number of code usages of each module to each other module it uses. It
is mostly a testing tool to verify that ast.go is working as intended.

Usage:

```
go install github.com/jadekler/lean/internal/cmd/countmoduleusages && \
  go mod graph | countmoduleusages
```