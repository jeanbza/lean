# modulecount

modulecount counts the number of references from module1@version to
module2@version.

Run:

On cache: `cd internal/cmd/modulecount && go run .`.
On lean: `cd internal/cmd/modulecount && go run .`.
On some module on fs:

```
cd internal/cmd/modulecount
go build .
cd mymodule
~/wherever/lean/internal/cmd/modulecount
```