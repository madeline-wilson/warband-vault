package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"warband-vault/internal/buildinfo"
	"warband-vault/internal/platform"
	"warband-vault/internal/update"
)

func main() {
	version := flag.Bool("version", false, "print launcher version")
	installRoot := flag.String("install-root", "", "installation root containing state/current.json")
	flag.Parse()
	if *version {
		fmt.Printf("warband-vault-launcher %s\n", buildinfo.Version)
		return
	}
	root := *installRoot
	if root == "" {
		executable, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve launcher path: %v\n", err)
			os.Exit(1)
		}
		root = filepath.Dir(executable)
	}
	state, err := update.CurrentState(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read current version state: %v\n", err)
		os.Exit(1)
	}
	target, err := update.ResolveExecutable(root, state)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve application executable: %v\n", err)
		os.Exit(1)
	}
	cmd := exec.Command(target, flag.Args()...)
	cmd.Env = append(os.Environ(), platform.InstallRootEnv+"="+root)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warband Vault exited with an error: %v\n", err)
		os.Exit(1)
	}
}
