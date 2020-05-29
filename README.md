# lean

lean helps users understand their module dependency graph, and find low-cost,
high-value cuts that can be made to reduce the dependency graph size.

![](https://user-images.githubusercontent.com/3584893/83226438-42908300-a13f-11ea-8082-7e2dbe7193ca.png)

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

# Appendix

### This is an MVP and is not production ready

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

### Immediate TODO

re: https://fasterthanli.me/blog/2020/i-want-off-mr-golangs-wild-ride/

Immediate things that need fixing:

- Module size should be cumulative to makethe sorting / figuring out which to
  cut actually useful.
- We should by default only show the direct dependencies of the root.
- Finding `cloud.google.com/go` is not working (I think because submodules).

Test by,

```
git clone https://github.com/getlantern/idletiming
cd idletiming
git checkout f356a3557cf3bd4c3f2fd10101bfb0728d0ce695
go mod graph | lean
```
