package main

import (
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"warband-vault/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "create":
		create(os.Args[2:])
	case "sign":
		sign(os.Args[2:])
	case "verify":
		verify(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func create(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	version := fs.String("version", "", "release version")
	minLauncher := fs.String("minimum-launcher-version", "v1.0.0", "minimum launcher version")
	out := fs.String("out", "update-manifest.json", "output manifest path")
	baseURL := fs.String("base-url", "", "base URL for artifacts")
	artifactFlags := multiFlag{}
	fs.Var(&artifactFlags, "artifact", "platform=path or platform=path=url")
	fs.Parse(args)
	if *version == "" || len(artifactFlags) == 0 {
		fmt.Fprintln(os.Stderr, "create requires --version and at least one --artifact")
		os.Exit(2)
	}
	manifest := update.Manifest{
		SchemaVersion:          update.SchemaVersion,
		Application:            update.ApplicationID,
		Version:                *version,
		PublishedAt:            time.Now().UTC(),
		MinimumLauncherVersion: *minLauncher,
		Releases:               map[string]update.ReleaseAsset{},
	}
	for _, spec := range artifactFlags {
		platform, path, assetURL, err := parseArtifact(spec, *baseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse artifact: %v\n", err)
			os.Exit(2)
		}
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stat artifact %s: %v\n", path, err)
			os.Exit(1)
		}
		sha, err := update.SHA256FileHex(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hash artifact %s: %v\n", path, err)
			os.Exit(1)
		}
		manifest.Releases[platform] = update.ReleaseAsset{URL: assetURL, Size: info.Size(), SHA256: sha}
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode manifest: %v\n", err)
		os.Exit(1)
	}
	body = append(body, '\n')
	if err := os.WriteFile(*out, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write manifest: %v\n", err)
		os.Exit(1)
	}
}

func sign(args []string) {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	in := fs.String("in", "", "file to sign")
	out := fs.String("out", "", "signature output path")
	privateKeyB64 := fs.String("private-key-b64", os.Getenv("UPDATE_SIGNING_PRIVATE_KEY_B64"), "base64 Ed25519 private key")
	fs.Parse(args)
	if *in == "" || *privateKeyB64 == "" {
		fmt.Fprintln(os.Stderr, "sign requires --in and --private-key-b64 or UPDATE_SIGNING_PRIVATE_KEY_B64")
		os.Exit(2)
	}
	if *out == "" {
		*out = *in + ".sig"
	}
	privateKey, err := update.DecodePrivateKeyB64(*privateKeyB64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode private key: %v\n", err)
		os.Exit(1)
	}
	body, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read input: %v\n", err)
		os.Exit(1)
	}
	sig := update.SignDetached(body, privateKey)
	if err := os.WriteFile(*out, sig, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write signature: %v\n", err)
		os.Exit(1)
	}
}

func verify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	in := fs.String("in", "", "file to verify")
	sigPath := fs.String("sig", "", "signature path")
	publicKeyB64 := fs.String("public-key-b64", "", "base64 Ed25519 public key")
	parseManifest := fs.Bool("manifest", false, "parse as update manifest after signature verification")
	fs.Parse(args)
	if *in == "" || *publicKeyB64 == "" {
		fmt.Fprintln(os.Stderr, "verify requires --in and --public-key-b64")
		os.Exit(2)
	}
	if *sigPath == "" {
		*sigPath = *in + ".sig"
	}
	publicKey, err := update.DecodePublicKeyB64(*publicKeyB64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode public key: %v\n", err)
		os.Exit(1)
	}
	body, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read input: %v\n", err)
		os.Exit(1)
	}
	sig, err := os.ReadFile(*sigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read signature: %v\n", err)
		os.Exit(1)
	}
	if *parseManifest {
		if _, err := update.ParseVerifiedManifest(body, sig, publicKey); err != nil {
			fmt.Fprintf(os.Stderr, "verify manifest: %v\n", err)
			os.Exit(1)
		}
	} else if err := update.VerifyDetached(body, sig, ed25519.PublicKey(publicKey)); err != nil {
		fmt.Fprintf(os.Stderr, "verify signature: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("signature verified")
}

func parseArtifact(spec, baseURL string) (string, string, string, error) {
	parts := strings.Split(spec, "=")
	if len(parts) != 2 && len(parts) != 3 {
		return "", "", "", fmt.Errorf("artifact must be platform=path or platform=path=url")
	}
	platform := parts[0]
	path := parts[1]
	assetURL := ""
	if len(parts) == 3 {
		assetURL = parts[2]
	} else if baseURL != "" {
		u, err := url.JoinPath(baseURL, filepath.Base(path))
		if err != nil {
			return "", "", "", err
		}
		assetURL = u
	} else {
		return "", "", "", fmt.Errorf("missing URL for artifact %s", platform)
	}
	return platform, path, assetURL, nil
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: manifest-tool create|sign|verify [flags]")
}
