package export

import (
	"bytes"
	"strings"
	"testing"

	"warband-vault/internal/campaign"
	"warband-vault/internal/character"
)

func TestJSONRoundTrip(t *testing.T) {
	source := campaign.ExampleBlackwaterExpedition()
	var buf bytes.Buffer
	if err := WriteJSON(&buf, &source); err != nil {
		t.Fatal(err)
	}
	got, err := ReadJSON(&buf, MaxImportSize)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != source.Name || len(got.Characters) != len(source.Characters) {
		t.Fatalf("unexpected round trip: %#v", got)
	}
}

func TestImportRejectsUnsupportedSchema(t *testing.T) {
	_, err := ReadJSON(strings.NewReader(`{"schema_version":99,"campaign":{"name":"x"}}`), MaxImportSize)
	if err == nil {
		t.Fatal("expected unsupported schema error")
	}
}

func TestImportSizeLimit(t *testing.T) {
	_, err := ReadJSON(strings.NewReader(`{"schema_version":1,"campaign":{"name":"x"}}`), 4)
	if err == nil {
		t.Fatal("expected size limit error")
	}
}

func TestRosterHTMLEscapesUserValues(t *testing.T) {
	c := &campaign.Campaign{
		Name: "<script>alert(1)</script>",
		Characters: []character.Character{
			{Name: "Mara", Notes: "<b>bold</b>"},
		},
	}
	var buf bytes.Buffer
	if err := WriteRosterHTML(&buf, c); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if strings.Contains(html, "<script>") || strings.Contains(html, "<b>bold</b>") {
		t.Fatalf("expected escaped HTML, got %s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("expected escaped script tag, got %s", html)
	}
}
