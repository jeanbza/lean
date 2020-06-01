package internal

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// readableModCache copies moduleRootPath sources to /tmp, since $GOPATH/pkg/mod
// is non-editable. It's useful when you need to do things like `go list` on a
// module, which apparently modifies the go.mod, which is then blocked by the
// readonly property.
//
// Once cleanup is called, the returned readableCopy is removed.
func readableModCache(moduleRootPath string) (readableCopy string, cleanup func(), _ error) {
	// Copy $GOPATH/mod/pkg/moduleRootPath to /tmp because we can't run
	// `go list` in $GOPATH due to -mod=readonly restriction.
	tmpdir, err := ioutil.TempDir("", "list-tmp")
	if err != nil {
		return "", nil, err
	}

	newDir := tmpdir + "/listme"
	cpCMD := exec.Command("cp", "-R", moduleRootPath, newDir)
	cpCMD.Stderr = os.Stderr
	if err := cpCMD.Run(); err != nil {
		return "", nil, err
	}

	// Set permissions so that we can modify go.mod / go.sum. (well, so that
	// go list can)
	//
	// NOTE: This won't work on windows, will it? 0777 looks very linux
	// specific.
	if err := os.Chmod(newDir, 0777); err != nil {
		return "", nil, err
	}
	if err := os.Chmod(filepath.Join(newDir, "go.mod"), 0777); err != nil {
		return "", nil, err
	}
	touchCMD := exec.Command("touch", "go.sum")
	touchCMD.Stderr = os.Stderr
	touchCMD.Dir = newDir
	if err := touchCMD.Run(); err != nil {
		return "", nil, err
	}
	if err := os.Chmod(filepath.Join(newDir, "go.sum"), 0777); err != nil {
		return "", nil, err
	}

	return newDir, func() {
		if err := removeContents(newDir); err != nil {
			log.Printf("warning: failed to remove %s: %s", newDir, err)
		}
	}, nil
}

// removeContents deletes the specified dir and all contents recursively.
//
// TODO(deklerk): This is hacky and should use os.RemoveAll and os.Chmod, but,
// those were kind of hard to string together when I tried (trying to chmod
// everything file-by-file, since there's no recursive Chmod; etc).
func removeContents(dir string) error {
	chmodCMD := exec.Command("chmod", "-R", "0777", dir)
	chmodCMD.Stdout = os.Stdout
	chmodCMD.Stderr = os.Stderr
	if err := chmodCMD.Run(); err != nil {
		return err
	}

	rmCMD := exec.Command("rm", "-rf", dir)
	rmCMD.Stdout = os.Stdout
	rmCMD.Stderr = os.Stderr
	return rmCMD.Run()
}

func filesInDir(dir string) ([]string, error) {
	files, err := ioutil.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var r []string
	for _, f := range files {
		r = append(r, f.Name())
	}
	return r, nil
}
