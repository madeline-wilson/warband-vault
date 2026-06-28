package update

import (
	"fmt"
	"os"
	"path/filepath"
)

type Lock struct {
	path string
	file *os.File
}

func AcquireLock(root string) (*Lock, error) {
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	path := filepath.Join(stateDir, "update.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another updater is active")
		}
		return nil, fmt.Errorf("acquire update lock: %w", err)
	}
	return &Lock{path: path, file: file}, nil
}

func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	var first error
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			first = err
		}
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) && first == nil {
		first = err
	}
	return first
}
