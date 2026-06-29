package main

import "testing"

func TestParseArgsPassesApplicationFlags(t *testing.T) {
	opts, appArgs, err := parseArgs([]string{"--install-root", "/tmp/app", "--smoke-test", "--data-dir", "/tmp/data"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.installRoot != "/tmp/app" {
		t.Fatalf("unexpected install root %q", opts.installRoot)
	}
	want := []string{"--smoke-test", "--data-dir", "/tmp/data"}
	if len(appArgs) != len(want) {
		t.Fatalf("expected %v, got %v", want, appArgs)
	}
	for i := range want {
		if appArgs[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, appArgs)
		}
	}
}

func TestParseArgsSupportsSeparator(t *testing.T) {
	opts, appArgs, err := parseArgs([]string{"--install-root=/tmp/app", "--", "--version"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.installRoot != "/tmp/app" {
		t.Fatalf("unexpected install root %q", opts.installRoot)
	}
	if len(appArgs) != 1 || appArgs[0] != "--version" {
		t.Fatalf("expected app args after separator, got %v", appArgs)
	}
}
