# lean

lean helps users understand their module dependency graph, and find low-cost,
high-value cuts that can be made to reduce the dependency graph size.

## Running

```
go install github.com/jadekler/lean
cd /path/to/my/project
go mod graph | lean
```

## Developing

Install and run (for development of lean):

```
go generate github.com/jadekler/lean/static && \
go install github.com/jadekler/lean && \
go mod graph | lean
```