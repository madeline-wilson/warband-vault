package persistence

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"warband-vault/internal/campaign"
	"warband-vault/internal/character"
	"warband-vault/internal/config"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	paths, err := config.ResolvePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(context.Background(), paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	})
	if paths.Database != filepath.Join(root, "warband-vault.db") {
		t.Fatalf("database path escaped test root: %s", paths.Database)
	}
	return store
}

func TestCampaignCRUD(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	c := &campaign.Campaign{Name: "The Blackwater Expedition", SystemName: "Generic Skirmish"}
	if err := store.Campaigns.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	c.Treasury = 42
	if err := store.Campaigns.Update(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := store.Campaigns.FindByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != c.Name || got.Treasury != 42 {
		t.Fatalf("unexpected campaign: %#v", got)
	}
	list, err := store.Campaigns.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one campaign, got %d", len(list))
	}
	if err := store.Campaigns.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Campaigns.FindByID(ctx, c.ID); !errors.Is(err, campaign.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCharacterCRUD(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	c := &campaign.Campaign{Name: "Company of Ash"}
	if err := store.Campaigns.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	ch := &character.Character{
		CampaignID: c.ID,
		Name:       "Mara",
		Role:       "Scout",
		Level:      2,
		Equipment:  []character.EquipmentItem{{Name: "Lantern", Quantity: 1}},
		Traits:     []character.Trait{{Name: "Sure-footed"}},
		Injuries:   []character.Injury{{Name: "Old scar"}},
		CustomFields: map[string]string{
			"oath": "Find the ford",
		},
	}
	if err := store.Characters.Create(ctx, ch); err != nil {
		t.Fatal(err)
	}
	ch.Experience = 7
	ch.CustomFields["oath"] = "Hold the ford"
	if err := store.Characters.Update(ctx, ch); err != nil {
		t.Fatal(err)
	}
	got, err := store.Characters.FindByID(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Experience != 7 || got.CustomFields["oath"] != "Hold the ford" {
		t.Fatalf("unexpected character: %#v", got)
	}
	if len(got.Equipment) != 1 || len(got.Traits) != 1 || len(got.Injuries) != 1 {
		t.Fatalf("expected nested records, got %#v", got)
	}
}

func TestImportCampaignAssignsNewIDOnCollision(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	original := &campaign.Campaign{Name: "Blackwater"}
	if err := store.Campaigns.Create(ctx, original); err != nil {
		t.Fatal(err)
	}
	imported, err := store.ImportCampaign(ctx, &campaign.Campaign{ID: original.ID, Name: "Blackwater Copy"})
	if err != nil {
		t.Fatal(err)
	}
	if imported.ID == original.ID {
		t.Fatal("expected import collision to assign a new id")
	}
}

func TestImportCampaignRollsBackMalformedCharacter(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_, err := store.ImportCampaign(ctx, &campaign.Campaign{
		Name: "Broken",
		Characters: []character.Character{
			{Name: "Good"},
			{Name: "Bad", Level: -1},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	list, listErr := store.Campaigns.List(ctx, true)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(list) != 0 {
		t.Fatalf("expected rollback, found %d campaigns", len(list))
	}
}
