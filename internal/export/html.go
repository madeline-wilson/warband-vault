package export

import (
	"fmt"
	"html/template"
	"io"
	"strings"

	"warband-vault/internal/campaign"
	"warband-vault/internal/character"
	"warband-vault/internal/validation"
)

var rosterTemplate = template.Must(template.New("roster").Funcs(template.FuncMap{
	"joinEquipment": func(items []character.EquipmentItem) string {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			label := item.Name
			if item.Quantity > 1 {
				label = fmt.Sprintf("%s x%d", item.Name, item.Quantity)
			}
			if item.Notes != "" {
				label += " (" + item.Notes + ")"
			}
			parts = append(parts, label)
		}
		return strings.Join(parts, ", ")
	},
	"joinTraits": func(items []character.Trait) string {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			label := item.Name
			if item.Notes != "" {
				label += " (" + item.Notes + ")"
			}
			parts = append(parts, label)
		}
		return strings.Join(parts, ", ")
	},
	"joinInjuries": func(items []character.Injury) string {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			label := item.Name
			if item.Recovered {
				label += " (recovered)"
			}
			if item.Notes != "" {
				label += " (" + item.Notes + ")"
			}
			parts = append(parts, label)
		}
		return strings.Join(parts, ", ")
	},
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{.Name}} roster</title>
<style>
:root { color-scheme: light; font-family: ui-serif, Georgia, "Times New Roman", serif; }
body { margin: 2rem; color: #1f2933; }
h1 { margin-bottom: .15rem; }
.meta { color: #52606d; margin-bottom: 1.5rem; }
.description { margin-bottom: 1.5rem; white-space: pre-wrap; }
table { border-collapse: collapse; width: 100%; font-size: 11pt; }
th, td { border: 1px solid #9aa5b1; padding: .45rem; vertical-align: top; text-align: left; }
th { background: #eef2f7; }
.notes { white-space: pre-wrap; }
@media print {
	body { margin: .5in; }
	table { page-break-inside: auto; }
	tr { page-break-inside: avoid; page-break-after: auto; }
}
</style>
</head>
<body>
<h1>{{.Name}}</h1>
<div class="meta">{{.SystemName}} &bull; Treasury: {{.Treasury}}</div>
<div class="description">{{.Description}}</div>
<table>
<thead>
<tr>
<th>Name</th><th>Role</th><th>Lvl</th><th>XP</th><th>Health</th><th>Move</th><th>Armor</th><th>Equipment</th><th>Traits</th><th>Injuries</th><th>Notes</th>
</tr>
</thead>
<tbody>
{{range .Characters}}
<tr>
<td>{{.Name}}</td>
<td>{{.Role}}</td>
<td>{{.Level}}</td>
<td>{{.Experience}}</td>
<td>{{.Health}}</td>
<td>{{.Movement}}</td>
<td>{{.Armor}}</td>
<td>{{joinEquipment .Equipment}}</td>
<td>{{joinTraits .Traits}}</td>
<td>{{joinInjuries .Injuries}}</td>
<td class="notes">{{.Notes}}{{range $key, $value := .CustomFields}}
{{$key}}: {{$value}}{{end}}</td>
</tr>
{{end}}
</tbody>
</table>
</body>
</html>
`))

func WriteRosterHTML(w io.Writer, c *campaign.Campaign) error {
	if err := validation.ValidateCampaign(c); err != nil {
		return err
	}
	if err := rosterTemplate.Execute(w, c); err != nil {
		return fmt.Errorf("render printable roster: %w", err)
	}
	return nil
}
