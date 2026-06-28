package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

type State struct {
	Version            string `json:"version"`
	RelativeExecutable string `json:"relative_executable"`
}

func ReadState(root, name string) (State, error) {
	body, err := os.ReadFile(filepath.Join(root, "state", name))
	if err != nil {
		return State{}, fmt.Errorf("read state %s: %w", name, err)
	}
	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return State{}, fmt.Errorf("decode state %s: %w", name, err)
	}
	if err := ValidateState(root, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func CurrentState(root string) (State, error) {
	return ReadState(root, "current.json")
}

func ValidateState(root string, state State) error {
	if normalizeVersion(state.Version) == "" || !semverIsValid(state.Version) {
		return fmt.Errorf("state version is invalid")
	}
	if state.RelativeExecutable == "" {
		return fmt.Errorf("state executable is required")
	}
	if filepath.IsAbs(state.RelativeExecutable) || filepath.VolumeName(state.RelativeExecutable) != "" || strings.HasPrefix(state.RelativeExecutable, `\\`) {
		return fmt.Errorf("state executable must be relative")
	}
	_, err := ResolveExecutable(root, state)
	return err
}

func ResolveExecutable(root string, state State) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve install root: %w", err)
	}
	execPath := filepath.Join(rootAbs, filepath.Clean(state.RelativeExecutable))
	rel, err := filepath.Rel(rootAbs, execPath)
	if err != nil {
		return "", fmt.Errorf("verify executable path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("state executable escapes install root")
	}
	return execPath, nil
}

func WriteCurrentState(root string, next State) error {
	if err := ValidateState(root, next); err != nil {
		return err
	}
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	currentPath := filepath.Join(stateDir, "current.json")
	if body, err := os.ReadFile(currentPath); err == nil {
		if err := writeAtomic(filepath.Join(stateDir, "previous.json"), body, 0o600); err != nil {
			return fmt.Errorf("write previous state: %w", err)
		}
	}
	body, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("encode current state: %w", err)
	}
	body = append(body, '\n')
	return writeAtomic(currentPath, body, 0o600)
}

func RollbackToPrevious(root string) (State, error) {
	previous, err := ReadState(root, "previous.json")
	if err != nil {
		return State{}, err
	}
	if err := WriteCurrentState(root, previous); err != nil {
		return State{}, fmt.Errorf("restore previous state: %w", err)
	}
	return previous, nil
}

func writeAtomic(path string, body []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	if _, err := file.Write(body); err != nil {
		file.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temporary file: %w", err)
	}
	return nil
}

func semverIsValid(v string) bool {
	return semver.IsValid(normalizeVersion(v))
}
