package update

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func signedManifest(t *testing.T, m Manifest) ([]byte, []byte, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw, SignDetached(raw, priv), pub
}

func validManifest() Manifest {
	return Manifest{
		SchemaVersion:          SchemaVersion,
		Application:            ApplicationID,
		Version:                "v1.1.0",
		PublishedAt:            time.Now().UTC(),
		MinimumLauncherVersion: "v1.0.0",
		Releases: map[string]ReleaseAsset{
			"linux-amd64": {
				URL:    "https://example.com/WarbandVault-linux-amd64.tar.gz",
				Size:   12,
				SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
		},
	}
}

func TestParseVerifiedManifestAcceptsValidSignature(t *testing.T) {
	raw, sig, pub := signedManifest(t, validManifest())
	manifest, err := ParseVerifiedManifest(raw, sig, pub)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "v1.1.0" {
		t.Fatalf("unexpected version %s", manifest.Version)
	}
}

func TestParseVerifiedManifestRejectsMissingSignature(t *testing.T) {
	raw, _, pub := signedManifest(t, validManifest())
	if _, err := ParseVerifiedManifest(raw, nil, pub); err == nil {
		t.Fatal("expected missing signature error")
	}
}

func TestParseVerifiedManifestRejectsInvalidSignature(t *testing.T) {
	raw, sig, pub := signedManifest(t, validManifest())
	sig[0] ^= 0xff
	if _, err := ParseVerifiedManifest(raw, sig, pub); err == nil {
		t.Fatal("expected invalid signature")
	}
}

func TestParseVerifiedManifestRejectsMalformedJSON(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("{")
	sig := SignDetached(raw, priv)
	if _, err := ParseVerifiedManifest(raw, sig, pub); err == nil {
		t.Fatal("expected malformed json error")
	}
}

func TestManifestValidationCases(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Manifest)
	}{
		{name: "application", edit: func(m *Manifest) { m.Application = "other" }},
		{name: "schema", edit: func(m *Manifest) { m.SchemaVersion = 99 }},
		{name: "version", edit: func(m *Manifest) { m.Version = "not semver" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.edit(&m)
			raw, sig, pub := signedManifest(t, m)
			if _, err := ParseVerifiedManifest(raw, sig, pub); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestManifestSelectCases(t *testing.T) {
	m := validManifest()
	_, err := m.Select(SelectionOptions{CurrentVersion: "v1.1.0", LauncherVersion: "v1.0.0", PlatformKey: "linux-amd64"})
	if !errors.Is(err, ErrNoUpdate) {
		t.Fatalf("expected no update, got %v", err)
	}
	_, err = m.Select(SelectionOptions{CurrentVersion: "v1.2.0", LauncherVersion: "v1.0.0", PlatformKey: "linux-amd64"})
	if err == nil {
		t.Fatal("expected downgrade rejection")
	}
	_, err = m.Select(SelectionOptions{CurrentVersion: "v1.0.0", LauncherVersion: "v0.9.0", PlatformKey: "linux-amd64"})
	if !errors.Is(err, ErrManualInstallRequired) {
		t.Fatalf("expected manual install error, got %v", err)
	}
	_, err = m.Select(SelectionOptions{CurrentVersion: "v1.0.0", LauncherVersion: "v1.0.0", PlatformKey: "darwin-arm64"})
	if err == nil {
		t.Fatal("expected missing platform entry")
	}
	selected, err := m.Select(SelectionOptions{CurrentVersion: "v1.0.0", LauncherVersion: "v1.0.0", PlatformKey: "linux-amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Version != "v1.1.0" {
		t.Fatalf("unexpected selection %#v", selected)
	}
}
