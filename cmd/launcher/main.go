package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"warband-vault/internal/buildinfo"
	"warband-vault/internal/platform"
	"warband-vault/internal/update"
)

func main() {
	opts, appArgs, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse launcher arguments: %v\n", err)
		os.Exit(2)
	}
	if opts.version {
		fmt.Printf("warband-vault-launcher %s\n", buildinfo.Version)
		return
	}
	root := opts.installRoot
	if root == "" {
		executable, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve launcher path: %v\n", err)
			os.Exit(1)
		}
		root = platform.InstallRootFromExecutable(executable)
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
	cmd := exec.Command(target, appArgs...)
	cmd.Env = append(os.Environ(), platform.InstallRootEnv+"="+root)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warband Vault exited with an error: %v\n", err)
		os.Exit(1)
	}
}

type launcherOptions struct {
	version     bool
	installRoot string
}

func parseArgs(args []string) (launcherOptions, []string, error) {
	var opts launcherOptions
	var appArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			appArgs = append(appArgs, args[i+1:]...)
			return opts, appArgs, nil
		case arg == "-version" || arg == "--version":
			opts.version = true
		case arg == "-install-root" || arg == "--install-root":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("%s requires a value", arg)
			}
			i++
			opts.installRoot = args[i]
		case strings.HasPrefix(arg, "-install-root="):
			opts.installRoot = strings.TrimPrefix(arg, "-install-root=")
		case strings.HasPrefix(arg, "--install-root="):
			opts.installRoot = strings.TrimPrefix(arg, "--install-root=")
		default:
			appArgs = append(appArgs, arg)
		}
	}
	return opts, appArgs, nil
}
