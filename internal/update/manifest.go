package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	ApplicationID = "warband-vault"
	SchemaVersion = 1
)

var (
	ErrNoUpdate              = errors.New("no update available")
	ErrManualInstallRequired = errors.New("manual installation required")
)

type ReleaseAsset struct {
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion          int                     `json:"schema_version"`
	Application            string                  `json:"application"`
	Version                string                  `json:"version"`
	PublishedAt            time.Time               `json:"published_at"`
	MinimumLauncherVersion string                  `json:"minimum_launcher_version"`
	Releases               map[string]ReleaseAsset `json:"releases"`
}

type SelectionOptions struct {
	CurrentVersion  string
	LauncherVersion string
	PlatformKey     string
	AllowDowngrade  bool
	AllowHTTP       bool
}

type Selection struct {
	Version string
	Asset   ReleaseAsset
}

func DecodePublicKeyB64(value string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key has %d bytes, expected %d", len(decoded), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(decoded), nil
}

func DecodePrivateKeyB64(value string) (ed25519.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("private key has %d bytes, expected %d", len(decoded), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(decoded), nil
}

func SignDetached(raw []byte, privateKey ed25519.PrivateKey) []byte {
	return ed25519.Sign(privateKey, raw)
}

func VerifyDetached(raw, sig []byte, publicKey ed25519.PublicKey) error {
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("signature has %d bytes, expected %d", len(sig), ed25519.SignatureSize)
	}
	if !ed25519.Verify(publicKey, raw, sig) {
		return errors.New("invalid signature")
	}
	return nil
}

func ParseVerifiedManifest(raw, sig []byte, publicKey ed25519.PublicKey) (*Manifest, error) {
	if len(sig) == 0 {
		return nil, errors.New("manifest signature is missing")
	}
	if err := VerifyDetached(raw, sig, publicKey); err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported manifest schema version %d", m.SchemaVersion)
	}
	if m.Application != ApplicationID {
		return fmt.Errorf("unexpected application identifier %q", m.Application)
	}
	if normalizeVersion(m.Version) == "" || !semver.IsValid(normalizeVersion(m.Version)) {
		return fmt.Errorf("invalid release version %q", m.Version)
	}
	if m.MinimumLauncherVersion != "" && !semver.IsValid(normalizeVersion(m.MinimumLauncherVersion)) {
		return fmt.Errorf("invalid minimum launcher version %q", m.MinimumLauncherVersion)
	}
	if len(m.Releases) == 0 {
		return errors.New("manifest contains no releases")
	}
	for platform, asset := range m.Releases {
		if strings.TrimSpace(platform) == "" {
			return errors.New("manifest contains empty platform key")
		}
		if err := asset.Validate(true); err != nil {
			return fmt.Errorf("validate release %s: %w", platform, err)
		}
	}
	return nil
}

func (a ReleaseAsset) Validate(allowHTTP bool) error {
	if a.URL == "" {
		return errors.New("release URL is required")
	}
	if a.Size <= 0 {
		return errors.New("release size must be positive")
	}
	if !isSHA256Hex(a.SHA256) {
		return errors.New("release sha256 is invalid")
	}
	parsed, err := url.Parse(a.URL)
	if err != nil {
		return fmt.Errorf("parse release URL: %w", err)
	}
	if parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http") {
		return fmt.Errorf("release URL must use HTTPS")
	}
	return nil
}

func (m Manifest) Select(opts SelectionOptions) (Selection, error) {
	if err := m.Validate(); err != nil {
		return Selection{}, err
	}
	current := normalizeVersion(opts.CurrentVersion)
	if current == "" || !semver.IsValid(current) {
		return Selection{}, fmt.Errorf("invalid current version %q", opts.CurrentVersion)
	}
	latest := normalizeVersion(m.Version)
	cmp := semver.Compare(latest, current)
	if cmp == 0 {
		return Selection{}, ErrNoUpdate
	}
	if cmp < 0 && !opts.AllowDowngrade {
		return Selection{}, fmt.Errorf("downgrade from %s to %s rejected", current, latest)
	}
	if m.MinimumLauncherVersion != "" {
		launcher := normalizeVersion(opts.LauncherVersion)
		if launcher == "" || !semver.IsValid(launcher) {
			return Selection{}, fmt.Errorf("invalid launcher version %q", opts.LauncherVersion)
		}
		if semver.Compare(launcher, normalizeVersion(m.MinimumLauncherVersion)) < 0 {
			return Selection{}, ErrManualInstallRequired
		}
	}
	asset, ok := m.Releases[opts.PlatformKey]
	if !ok {
		return Selection{}, fmt.Errorf("manifest missing platform entry %s", opts.PlatformKey)
	}
	if err := asset.Validate(opts.AllowHTTP); err != nil {
		return Selection{}, err
	}
	return Selection{Version: latest, Asset: asset}, nil
}

func SHA256FileHex(path string) (string, error) {
	sum, err := hashFile(path)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum[:]), nil
}

func SHA256BytesHex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
