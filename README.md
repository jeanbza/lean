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

## This is an MVP and is not production ready

This is an MVP. There are several parts of this that are not fully fleshed out.
An incomplete list:

- Shorten hashes.
- Always return sorted graph, so that graph is more consistently rendered same
way across edge adds/deletes.
- Calculate # vertices removed by cut per edge.
- Calculate # bytes removed by cut per edge.
- Calculate # usages of 'to' by 'from' per edge.
- Refactor connectedness algorithm to use dominator tree for better performance.
- Tests, especially around connectedness algorithms.
- Check $GOPATH, vendor, replace directives when calculating module size & ast.

## Immediate TODO

Immediate things that need fixing:

- Finding `golang.org/x/tools@v0.0.0-20190524140312-2c0ae7006135` is not working.
- Finding `cloud.google.com/go` is not working (I think because submodules).

Test by,

```
git clone https://github.com/getlantern/idletiming
cd idletiming
git checkout f356a3557cf3bd4c3f2fd10101bfb0728d0ce695
go mod graph | lean
```
