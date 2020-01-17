# lean

Running:

```
go install github.com/jadekler/lean
go mod graph | lean
```

Install and run (for development of lean):

```
go generate github.com/jadekler/lean/static && \
go install github.com/jadekler/lean && \
go mod graph | lean
```