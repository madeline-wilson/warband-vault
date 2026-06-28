package export

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"warband-vault/internal/campaign"
	"warband-vault/internal/validation"
)

const (
	SchemaVersion = 1
	MaxImportSize = 8 << 20
)

type Envelope struct {
	SchemaVersion int               `json:"schema_version"`
	ExportedAt    time.Time         `json:"exported_at"`
	Campaign      campaign.Campaign `json:"campaign"`
}

func WriteJSON(w io.Writer, c *campaign.Campaign) error {
	if err := validation.ValidateCampaign(c); err != nil {
		return err
	}
	envelope := Envelope{
		SchemaVersion: SchemaVersion,
		ExportedAt:    time.Now().UTC(),
		Campaign:      *c,
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(envelope); err != nil {
		return fmt.Errorf("encode campaign export: %w", err)
	}
	return nil
}

func ReadJSON(r io.Reader, maxBytes int64) (*campaign.Campaign, error) {
	if maxBytes <= 0 {
		maxBytes = MaxImportSize
	}
	limited := io.LimitReader(r, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read campaign import: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("campaign import exceeds %d bytes", maxBytes)
	}
	var envelope Envelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode campaign import: %w", err)
	}
	if envelope.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported campaign export schema version %d", envelope.SchemaVersion)
	}
	if err := validation.ValidateCampaign(&envelope.Campaign); err != nil {
		return nil, err
	}
	return &envelope.Campaign, nil
}
