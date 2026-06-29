package ui

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"warband-vault/assets"
	"warband-vault/internal/app"
	"warband-vault/internal/buildinfo"
	"warband-vault/internal/campaign"
	"warband-vault/internal/character"
	"warband-vault/internal/config"
	warbandexport "warband-vault/internal/export"
	"warband-vault/internal/platform"
	"warband-vault/internal/update"
)

type mainWindow struct {
	services    *app.Services
	fyneApp     fyne.App
	window      fyne.Window
	campaigns   []campaign.Campaign
	selected    *campaign.Campaign
	list        *widget.List
	listHost    *fyne.Container
	roster      *fyne.Container
	status      binding.String
	busy        bool
	suppress    bool
	checkOnOpen bool
}

func Run(services *app.Services) {
	a := fyneapp.NewWithID("com.warbandvault.app")
	a.Settings().SetTheme(newVaultTheme())
	w := a.NewWindow("Warband Vault")
	w.Resize(fyne.NewSize(1180, 760))
	m := &mainWindow{
		services: services,
		fyneApp:  a,
		window:   w,
		roster:   container.NewVBox(widget.NewLabel("")),
		status:   binding.NewString(),
	}
	m.checkOnOpen = services.Settings.UpdateCheckOnStartup
	_ = m.status.Set("Ready")
	m.build()
	m.loadCampaigns()
	w.ShowAndRun()
}

func (m *mainWindow) build() {
	m.list = m.newCampaignList()
	m.listHost = container.NewStack(m.list)
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.ContentAddIcon(), func() { m.showCampaignEditor(nil) }),
		widget.NewToolbarAction(theme.AccountIcon(), func() { m.showCharacterEditor(nil) }),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.UploadIcon(), m.importCampaign),
		widget.NewToolbarAction(theme.DownloadIcon(), m.exportCampaign),
		widget.NewToolbarAction(theme.DocumentPrintIcon(), m.printRoster),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.SettingsIcon(), m.showSettings),
		widget.NewToolbarAction(theme.ViewRefreshIcon(), m.checkForUpdates),
	)
	leftTitle := widget.NewLabelWithStyle("Campaigns", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	leftSubtitle := widget.NewLabel("warband registry")
	left := container.NewBorder(container.NewVBox(leftTitle, leftSubtitle, widget.NewSeparator()), nil, nil, nil, m.listHost)
	split := container.NewHSplit(left, container.NewVScroll(m.roster))
	split.Offset = 0.27
	status := widget.NewLabelWithData(m.status)
	m.window.SetContent(container.NewBorder(toolbar, status, nil, nil, split))
}

func (m *mainWindow) newCampaignList() *widget.List {
	m.list = widget.NewList(
		func() int { return len(m.campaigns) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id < 0 || id >= len(m.campaigns) {
				label.SetText("")
				return
			}
			c := m.campaigns[id]
			name := c.Name
			if c.Archived {
				name += " (archived)"
			}
			label.SetText(name)
		},
	)
	m.list.OnSelected = func(id widget.ListItemID) {
		if m.suppress {
			return
		}
		if id < 0 || id >= len(m.campaigns) {
			return
		}
		m.loadCampaign(m.campaigns[id].ID)
	}
	return m.list
}

func (m *mainWindow) loadCampaigns() {
	m.run("Loading campaigns", func(ctx context.Context) error {
		campaigns, selected, err := m.campaignSnapshot(ctx, "")
		if err != nil {
			return err
		}
		fyne.Do(func() {
			m.applySnapshot(campaigns, selected)
		})
		return nil
	})
}

func (m *mainWindow) loadCampaign(id string) {
	m.run("Loading roster", func(ctx context.Context) error {
		c, err := m.services.Store.Campaigns.FindByID(ctx, id)
		if err != nil {
			return err
		}
		fyne.Do(func() {
			m.selected = c
			m.renderRoster(c)
		})
		return nil
	})
}

func (m *mainWindow) renderEmpty() {
	title := widget.NewLabelWithStyle("No campaigns", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	prompt := widget.NewLabel("Open a new ledger, then start recruiting.")
	m.roster.Objects = []fyne.CanvasObject{
		m.sectionHeader("VAULT // EMPTY LEDGER", "No active campaign signal"),
		title,
		prompt,
		widget.NewButtonWithIcon("Create example campaign", theme.ContentAddIcon(), m.createExampleCampaign),
	}
	m.roster.Refresh()
}

func (m *mainWindow) renderRoster(c *campaign.Campaign) {
	title := widget.NewLabelWithStyle(c.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	meta := widget.NewLabel(fmt.Sprintf("%s    Treasury: %d crowns    Operators: %d", c.SystemName, c.Treasury, len(c.Characters)))
	description := widget.NewLabel(c.Description)
	description.Wrapping = fyne.TextWrapWord
	actions := container.NewHBox(
		widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() { m.showCampaignEditor(c) }),
		widget.NewButtonWithIcon("New character", theme.AccountIcon(), func() { m.showCharacterEditor(nil) }),
		widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), m.deleteSelectedCampaign),
	)
	objects := []fyne.CanvasObject{
		m.sectionHeader("WAR VAULT // ACTIVE CAMPAIGN", "rune-link stable"),
		container.NewBorder(nil, nil, nil, actions, container.NewVBox(title, meta, description)),
		widget.NewSeparator(),
	}
	for i := range c.Characters {
		ch := c.Characters[i]
		objects = append(objects, m.characterCard(&ch))
	}
	if len(c.Characters) == 0 {
		objects = append(objects, widget.NewLabel("Roster is empty"))
	}
	m.roster.Objects = objects
	m.roster.Refresh()
}

func (m *mainWindow) characterCard(ch *character.Character) fyne.CanvasObject {
	stats := fmt.Sprintf("Role: %s   Level: %d   XP: %d   Health: %d   Move: %d   Armor: %d", ch.Role, ch.Level, ch.Experience, ch.Health, ch.Movement, ch.Armor)
	notes := widget.NewLabel(ch.Notes)
	notes.Wrapping = fyne.TextWrapWord
	edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { m.showCharacterEditor(ch) })
	deleteButton := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { m.deleteCharacter(ch) })
	title := container.NewBorder(nil, nil, nil, container.NewHBox(edit, deleteButton), widget.NewLabelWithStyle(ch.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	body := container.NewVBox(
		title,
		widget.NewSeparator(),
		widget.NewLabel(stats),
		widget.NewLabel("Equipment: "+formatEquipment(ch.Equipment)),
		widget.NewLabel("Traits: "+formatTraits(ch.Traits)),
		widget.NewLabel("Injuries: "+formatInjuries(ch.Injuries)),
		notes,
	)
	return widget.NewCard("", "", body)
}

func (m *mainWindow) sectionHeader(title, status string) fyne.CanvasObject {
	titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	statusLabel := widget.NewLabel(status)
	bar := canvas.NewRectangle(color.NRGBA{R: 0x00, G: 0xf0, B: 0xff, A: 0xff})
	bar.SetMinSize(fyne.NewSize(4, 34))
	accent := canvas.NewRectangle(color.NRGBA{R: 0xd7, G: 0x9d, B: 0x3d, A: 0xff})
	accent.SetMinSize(fyne.NewSize(54, 2))
	return container.NewBorder(nil, accent, bar, nil, container.NewVBox(titleLabel, statusLabel))
}

func (m *mainWindow) showCampaignEditor(existing *campaign.Campaign) {
	c := &campaign.Campaign{}
	title := "New campaign"
	if existing != nil {
		copy := *existing
		c = &copy
		title = "Edit campaign"
	}
	name := widget.NewEntry()
	name.SetText(c.Name)
	system := widget.NewEntry()
	system.SetText(c.SystemName)
	description := widget.NewMultiLineEntry()
	description.SetText(c.Description)
	treasury := widget.NewEntry()
	treasury.SetText(strconv.Itoa(c.Treasury))
	archived := widget.NewCheck("", nil)
	archived.SetChecked(c.Archived)
	form := []*widget.FormItem{
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Game/system", system),
		widget.NewFormItem("Description", description),
		widget.NewFormItem("Treasury", treasury),
		widget.NewFormItem("Archived", archived),
	}
	dialog.ShowForm(title, "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		value, err := strconv.Atoi(strings.TrimSpace(treasury.Text))
		if err != nil {
			dialog.ShowError(fmt.Errorf("treasury must be a whole number"), m.window)
			return
		}
		c.Name = name.Text
		c.SystemName = system.Text
		c.Description = description.Text
		c.Treasury = value
		c.Archived = archived.Checked
		m.run("Saving campaign", func(ctx context.Context) error {
			if c.ID == "" {
				if err := m.services.Store.Campaigns.Create(ctx, c); err != nil {
					return err
				}
			} else if err := m.services.Store.Campaigns.Update(ctx, c); err != nil {
				return err
			}
			campaigns, selected, err := m.campaignSnapshot(ctx, c.ID)
			if err != nil {
				return err
			}
			fyne.Do(func() {
				m.applySnapshot(campaigns, selected)
			})
			return nil
		})
	}, m.window)
}

func (m *mainWindow) showCharacterEditor(existing *character.Character) {
	if m.selected == nil {
		dialog.ShowInformation("No campaign selected", "Select or create a campaign first.", m.window)
		return
	}
	ch := &character.Character{CampaignID: m.selected.ID, CustomFields: map[string]string{}}
	title := "New character"
	if existing != nil {
		copy := *existing
		copy.Equipment = append([]character.EquipmentItem(nil), existing.Equipment...)
		copy.Traits = append([]character.Trait(nil), existing.Traits...)
		copy.Injuries = append([]character.Injury(nil), existing.Injuries...)
		copy.CustomFields = cloneFields(existing.CustomFields)
		ch = &copy
		title = "Edit character"
	}
	name := entry(ch.Name)
	role := entry(ch.Role)
	level := entry(strconv.Itoa(ch.Level))
	xp := entry(strconv.Itoa(ch.Experience))
	health := entry(strconv.Itoa(ch.Health))
	movement := entry(strconv.Itoa(ch.Movement))
	armor := entry(strconv.Itoa(ch.Armor))
	notes := widget.NewMultiLineEntry()
	notes.SetText(ch.Notes)
	equipment := widget.NewMultiLineEntry()
	equipment.SetText(formatEquipmentLines(ch.Equipment))
	traits := widget.NewMultiLineEntry()
	traits.SetText(formatTraitLines(ch.Traits))
	injuries := widget.NewMultiLineEntry()
	injuries.SetText(formatInjuryLines(ch.Injuries))
	custom := widget.NewMultiLineEntry()
	custom.SetText(formatCustomLines(ch.CustomFields))
	items := []*widget.FormItem{
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Role", role),
		widget.NewFormItem("Level", level),
		widget.NewFormItem("Experience", xp),
		widget.NewFormItem("Health", health),
		widget.NewFormItem("Movement", movement),
		widget.NewFormItem("Armor", armor),
		widget.NewFormItem("Equipment", equipment),
		widget.NewFormItem("Traits", traits),
		widget.NewFormItem("Injuries", injuries),
		widget.NewFormItem("Custom fields", custom),
		widget.NewFormItem("Notes", notes),
	}
	dialog.ShowForm(title, "Save", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		values, err := parseNumbers(map[string]string{
			"level":      level.Text,
			"experience": xp.Text,
			"health":     health.Text,
			"movement":   movement.Text,
			"armor":      armor.Text,
		})
		if err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		ch.Name = name.Text
		ch.Role = role.Text
		ch.Level = values["level"]
		ch.Experience = values["experience"]
		ch.Health = values["health"]
		ch.Movement = values["movement"]
		ch.Armor = values["armor"]
		ch.Notes = notes.Text
		ch.Equipment = parseEquipment(equipment.Text)
		ch.Traits = parseTraits(traits.Text)
		ch.Injuries = parseInjuries(injuries.Text)
		ch.CustomFields = parseCustomFields(custom.Text)
		m.run("Saving character", func(ctx context.Context) error {
			if ch.ID == "" {
				if err := m.services.Store.Characters.Create(ctx, ch); err != nil {
					return err
				}
			} else if err := m.services.Store.Characters.Update(ctx, ch); err != nil {
				return err
			}
			selected, err := m.services.Store.Campaigns.FindByID(ctx, ch.CampaignID)
			if err != nil {
				return err
			}
			fyne.Do(func() { m.applySelectedCampaign(selected) })
			return nil
		})
	}, m.window)
}

func (m *mainWindow) importCampaign() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()
		c, err := warbandexport.ReadJSON(reader, warbandexport.MaxImportSize)
		if err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		m.run("Importing campaign", func(ctx context.Context) error {
			imported, err := m.services.Store.ImportCampaign(ctx, c)
			if err != nil {
				return err
			}
			campaigns, selected, err := m.campaignSnapshot(ctx, imported.ID)
			if err != nil {
				return err
			}
			fyne.Do(func() {
				m.applySnapshot(campaigns, selected)
			})
			return nil
		})
	}, m.window)
	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	fileDialog.Show()
}

func (m *mainWindow) exportCampaign() {
	if m.selected == nil {
		dialog.ShowInformation("No campaign selected", "Select a campaign to export.", m.window)
		return
	}
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()
		var buf bytes.Buffer
		if err := warbandexport.WriteJSON(&buf, m.selected); err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		if _, err := writer.Write(buf.Bytes()); err != nil {
			dialog.ShowError(err, m.window)
		}
	}, m.window)
	saveDialog.SetFileName(safeFileName(m.selected.Name) + ".json")
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	saveDialog.Show()
}

func (m *mainWindow) printRoster() {
	if m.selected == nil {
		dialog.ShowInformation("No campaign selected", "Select a campaign to print.", m.window)
		return
	}
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()
		var buf bytes.Buffer
		if err := warbandexport.WriteRosterHTML(&buf, m.selected); err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		if _, err := writer.Write(buf.Bytes()); err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		uri := writer.URI()
		if uri != nil && uri.Scheme() == "file" {
			openFile(uri.Path())
		}
	}, m.window)
	saveDialog.SetFileName(safeFileName(m.selected.Name) + "-roster.html")
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".html", ".htm"}))
	saveDialog.Show()
}

func (m *mainWindow) showSettings() {
	checkStartup := widget.NewCheck("", nil)
	checkStartup.SetChecked(m.services.Settings.UpdateCheckOnStartup)
	manifestURL := widget.NewEntry()
	manifestURL.SetText(m.services.Settings.UpdateManifestURL)
	version := widget.NewLabel(fmt.Sprintf("%s\nCommit: %s\nBuilt: %s\nChannel: %s", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate, buildinfo.Channel))
	createExample := widget.NewButtonWithIcon("Create example campaign", theme.ContentAddIcon(), m.createExampleCampaign)
	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Version", version),
			widget.NewFormItem("Check on startup", checkStartup),
			widget.NewFormItem("Manifest URL", manifestURL),
		),
		createExample,
	)
	dialog.ShowCustomConfirm("Settings", "Save", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		settings := config.Settings{UpdateCheckOnStartup: checkStartup.Checked, UpdateManifestURL: manifestURL.Text}
		if err := config.Save(m.services.Paths.ConfigFile, settings); err != nil {
			dialog.ShowError(err, m.window)
			return
		}
		m.services.Settings = settings
	}, m.window)
}

func (m *mainWindow) checkForUpdates() {
	m.checkForUpdatesPrompt(false)
}

func (m *mainWindow) checkForUpdatesOnOpen() {
	m.checkForUpdatesPrompt(true)
}

func (m *mainWindow) checkForUpdatesPrompt(quiet bool) {
	settings := m.services.Settings
	m.run("Checking for updates", func(ctx context.Context) error {
		keyBytes, err := assets.Files.ReadFile("update_public_key.txt")
		if err != nil {
			if quiet {
				m.services.Logger.Warn("startup update check failed", "error", err)
				return nil
			}
			return err
		}
		publicKey, err := update.DecodePublicKeyB64(string(keyBytes))
		if err != nil {
			if quiet {
				m.services.Logger.Warn("startup update check failed", "error", err)
				return nil
			}
			return err
		}
		allowHTTP := isLocalHTTP(settings.UpdateManifestURL)
		downloader := update.NewDownloader(20*time.Second, m.services.Logger)
		manifest, _, err := downloader.FetchSignedManifest(ctx, settings.UpdateManifestURL, publicKey)
		if err != nil {
			if quiet {
				m.services.Logger.Warn("startup update check failed", "error", err)
				return nil
			}
			return err
		}
		selection, err := manifest.Select(update.SelectionOptions{
			CurrentVersion:  buildinfo.Version,
			LauncherVersion: buildinfo.Version,
			PlatformKey:     platform.PlatformKey(),
			AllowHTTP:       allowHTTP,
		})
		if err != nil {
			if err == update.ErrNoUpdate {
				if !quiet {
					fyne.Do(func() { dialog.ShowInformation("Updates", "Warband Vault is up to date.", m.window) })
				}
				return nil
			}
			if quiet {
				m.services.Logger.Warn("startup update check failed", "error", err)
				return nil
			}
			return err
		}
		fyne.Do(func() { m.showUpdateDialog(selection, allowHTTP) })
		return nil
	})
}

func (m *mainWindow) showUpdateDialog(selection update.Selection, allowHTTP bool) {
	content := widget.NewLabel(fmt.Sprintf("Available version: %s\nDownload size: %d bytes", selection.Version, selection.Asset.Size))
	dialog.ShowCustomConfirm("Update available", "Install and restart", "Remind me later", content, func(ok bool) {
		if !ok {
			return
		}
		installRoot := os.Getenv(platform.InstallRootEnv)
		if installRoot == "" {
			dialog.ShowInformation("Manual installation required", "This copy was not launched from a versioned installation root.", m.window)
			return
		}
		m.run("Installing update", func(ctx context.Context) error {
			if err := update.CheckInstallRootWritable(installRoot); err != nil {
				return err
			}
			downloader := update.NewDownloader(30*time.Second, m.services.Logger)
			packagePath, err := downloader.DownloadArtifact(ctx, selection.Asset.URL, filepath.Join(installRoot, "downloads"), selection.Asset.Size, selection.Asset.SHA256, allowHTTP)
			if err != nil {
				return err
			}
			if _, err := update.StageVersion(ctx, update.InstallOptions{InstallRoot: installRoot, Version: selection.Version, PackagePath: packagePath}); err != nil {
				return err
			}
			launcher := filepath.Join(installRoot, platform.ExecutableName("warband-vault-launcher"))
			cmd := exec.Command(launcher)
			cmd.Env = append(os.Environ(), platform.InstallRootEnv+"="+installRoot)
			if err := cmd.Start(); err != nil {
				return err
			}
			fyne.Do(func() { m.fyneApp.Quit() })
			return nil
		})
	}, m.window)
}

func (m *mainWindow) createExampleCampaign() {
	m.run("Creating example campaign", func(ctx context.Context) error {
		example, err := m.services.CreateExampleCampaign(ctx)
		if err != nil {
			return err
		}
		campaigns, selected, err := m.campaignSnapshot(ctx, example.ID)
		if err != nil {
			return err
		}
		fyne.Do(func() {
			m.applySnapshot(campaigns, selected)
		})
		return nil
	})
}

func (m *mainWindow) deleteSelectedCampaign() {
	if m.selected == nil {
		return
	}
	id := m.selected.ID
	dialog.ShowConfirm("Delete campaign", "Delete "+m.selected.Name+"?", func(ok bool) {
		if !ok {
			return
		}
		m.run("Deleting campaign", func(ctx context.Context) error {
			if err := m.services.Store.Campaigns.Delete(ctx, id); err != nil {
				return err
			}
			campaigns, selected, err := m.campaignSnapshot(ctx, "")
			if err != nil {
				return err
			}
			fyne.Do(func() {
				m.applySnapshot(campaigns, selected)
			})
			return nil
		})
	}, m.window)
}

func (m *mainWindow) deleteCharacter(ch *character.Character) {
	dialog.ShowConfirm("Delete character", "Delete "+ch.Name+"?", func(ok bool) {
		if !ok {
			return
		}
		campaignID := m.selected.ID
		m.run("Deleting character", func(ctx context.Context) error {
			if err := m.services.Store.Characters.Delete(ctx, ch.ID); err != nil {
				return err
			}
			selected, err := m.services.Store.Campaigns.FindByID(ctx, campaignID)
			if err != nil {
				return err
			}
			fyne.Do(func() { m.applySelectedCampaign(selected) })
			return nil
		})
	}, m.window)
}

func (m *mainWindow) campaignSnapshot(ctx context.Context, selectedID string) ([]campaign.Campaign, *campaign.Campaign, error) {
	campaigns, err := m.services.Store.Campaigns.List(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	if selectedID == "" && len(campaigns) > 0 {
		selectedID = campaigns[0].ID
	}
	if selectedID == "" {
		return campaigns, nil, nil
	}
	selected, err := m.services.Store.Campaigns.FindByID(ctx, selectedID)
	if err != nil {
		return nil, nil, err
	}
	return campaigns, selected, nil
}

func (m *mainWindow) applySnapshot(campaigns []campaign.Campaign, selected *campaign.Campaign) {
	m.suppress = true
	m.list.UnselectAll()
	m.suppress = false
	m.campaigns = campaigns
	m.replaceCampaignList()
	if selected != nil {
		m.applySelectedCampaign(selected)
		return
	}
	m.selected = nil
	m.renderEmpty()
}

func (m *mainWindow) applySelectedCampaign(selected *campaign.Campaign) {
	m.selected = selected
	if selected == nil {
		m.renderEmpty()
		return
	}
	m.selectCampaignInList(selected.ID)
	m.renderRoster(selected)
}

func (m *mainWindow) selectCampaignInList(id string) {
	for i, c := range m.campaigns {
		if c.ID != id {
			continue
		}
		m.suppress = true
		m.list.Select(widget.ListItemID(i))
		m.suppress = false
		m.list.RefreshItem(widget.ListItemID(i))
		return
	}
	m.suppress = true
	m.list.UnselectAll()
	m.suppress = false
}

func (m *mainWindow) replaceCampaignList() {
	if m.listHost == nil {
		m.list = m.newCampaignList()
		return
	}
	m.list = m.newCampaignList()
	m.listHost.Objects = []fyne.CanvasObject{m.list}
	m.listHost.Refresh()
}

func (m *mainWindow) run(label string, fn func(context.Context) error) {
	if m.busy {
		return
	}
	m.busy = true
	_ = m.status.Set(label + "...")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		err := fn(ctx)
		fyne.Do(func() {
			m.busy = false
			if err != nil {
				_ = m.status.Set("Error")
				m.services.Logger.Error("operation failed", "operation", label, "error", err)
				dialog.ShowError(err, m.window)
				return
			}
			_ = m.status.Set("Ready")
			if m.checkOnOpen {
				m.checkOnOpen = false
				m.checkForUpdatesOnOpen()
			}
		})
	}()
}

func entry(value string) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(value)
	return e
}

func cloneFields(fields map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range fields {
		out[key] = value
	}
	return out
}

func parseNumbers(values map[string]string) (map[string]int, error) {
	out := map[string]int{}
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			out[key] = 0
			continue
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("%s must be a whole number", key)
		}
		out[key] = parsed
	}
	return out, nil
}

func parseEquipment(text string) []character.EquipmentItem {
	var items []character.EquipmentItem
	for _, line := range nonEmptyLines(text) {
		parts := splitPipe(line, 3)
		quantity := 1
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				quantity = parsed
			}
		}
		notes := ""
		if len(parts) > 2 {
			notes = parts[2]
		}
		items = append(items, character.EquipmentItem{Name: parts[0], Quantity: quantity, Notes: notes})
	}
	return items
}

func parseTraits(text string) []character.Trait {
	var items []character.Trait
	for _, line := range nonEmptyLines(text) {
		parts := splitPipe(line, 2)
		notes := ""
		if len(parts) > 1 {
			notes = parts[1]
		}
		items = append(items, character.Trait{Name: parts[0], Notes: notes})
	}
	return items
}

func parseInjuries(text string) []character.Injury {
	var items []character.Injury
	for _, line := range nonEmptyLines(text) {
		parts := splitPipe(line, 3)
		recovered := len(parts) > 1 && strings.EqualFold(strings.TrimSpace(parts[1]), "recovered")
		notes := ""
		if len(parts) > 2 {
			notes = parts[2]
		}
		items = append(items, character.Injury{Name: parts[0], Recovered: recovered, Notes: notes})
	}
	return items
}

func parseCustomFields(text string) map[string]string {
	fields := map[string]string{}
	for _, line := range nonEmptyLines(text) {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return fields
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitPipe(line string, n int) []string {
	parts := strings.SplitN(line, "|", n)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func formatEquipment(items []character.EquipmentItem) string {
	if len(items) == 0 {
		return ""
	}
	var parts []string
	for _, item := range items {
		label := item.Name
		if item.Quantity > 1 {
			label += " x" + strconv.Itoa(item.Quantity)
		}
		if item.Notes != "" {
			label += " (" + item.Notes + ")"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

func formatTraits(items []character.Trait) string {
	var parts []string
	for _, item := range items {
		label := item.Name
		if item.Notes != "" {
			label += " (" + item.Notes + ")"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

func formatInjuries(items []character.Injury) string {
	var parts []string
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
}

func formatEquipmentLines(items []character.EquipmentItem) string {
	var lines []string
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s | %d | %s", item.Name, item.Quantity, item.Notes))
	}
	return strings.Join(lines, "\n")
}

func formatTraitLines(items []character.Trait) string {
	var lines []string
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s | %s", item.Name, item.Notes))
	}
	return strings.Join(lines, "\n")
}

func formatInjuryLines(items []character.Injury) string {
	var lines []string
	state := ""
	for _, item := range items {
		if item.Recovered {
			state = "recovered"
		} else {
			state = "active"
		}
		lines = append(lines, fmt.Sprintf("%s | %s | %s", item.Name, state, item.Notes))
	}
	return strings.Join(lines, "\n")
}

func formatCustomLines(fields map[string]string) string {
	var lines []string
	for key, value := range fields {
		lines = append(lines, key+"="+value)
	}
	return strings.Join(lines, "\n")
}

func safeFileName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "campaign"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func isLocalHTTP(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" {
		return false
	}
	host := strings.Split(u.Host, ":")[0]
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func openFile(path string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", path).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	default:
		_ = exec.Command("xdg-open", path).Start()
	}
}
