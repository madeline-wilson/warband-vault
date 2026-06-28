package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRejectsAbsoluteExecutable(t *testing.T) {
	err := ValidateState(t.TempDir(), State{Version: "v1.0.0", RelativeExecutable: filepath.Join(string(os.PathSeparator), "tmp", "app")})
	if err == nil {
		t.Fatal("expected absolute path rejection")
	}
}

func TestWriteAndReadCurrentState(t *testing.T) {
	root := t.TempDir()
	state := State{Version: "v1.0.0", RelativeExecutable: "versions/v1.0.0/warband-vault"}
	if err := WriteCurrentState(root, state); err != nil {
		t.Fatal(err)
	}
	got, err := CurrentState(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != state.Version {
		t.Fatalf("unexpected state %#v", got)
	}
}

func TestRollbackToPrevious(t *testing.T) {
	root := t.TempDir()
	first := State{Version: "v1.0.0", RelativeExecutable: "versions/v1.0.0/warband-vault"}
	second := State{Version: "v1.1.0", RelativeExecutable: "versions/v1.1.0/warband-vault"}
	if err := WriteCurrentState(root, first); err != nil {
		t.Fatal(err)
	}
	if err := WriteCurrentState(root, second); err != nil {
		t.Fatal(err)
	}
	restored, err := RollbackToPrevious(root)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Version != first.Version {
		t.Fatalf("expected rollback to %s, got %#v", first.Version, restored)
	}
	current, err := CurrentState(root)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != first.Version {
		t.Fatalf("expected current rollback, got %#v", current)
	}
}

func TestAcquireLockExcludesSecondUpdater(t *testing.T) {
	root := t.TempDir()
	lock, err := AcquireLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(root); err == nil {
		t.Fatal("expected second lock to fail")
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if lock, err = AcquireLock(root); err != nil {
		t.Fatal(err)
	}
	lock.Release()
}

func TestWaitForHealth(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "v1.ok")
	go func() {
		time.Sleep(20 * time.Millisecond)
		os.WriteFile(marker, []byte("ok"), 0o644)
	}()
	if err := WaitForHealth(context.Background(), marker, time.Second); err != nil {
		t.Fatal(err)
	}
	if err := WaitForHealth(context.Background(), filepath.Join(t.TempDir(), "missing"), 10*time.Millisecond); err == nil {
		t.Fatal("expected timeout")
	}
}
