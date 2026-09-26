package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func testModel(provider, id string) *model.Model {
	return &model.Model{
		ID:            id,
		Name:          id,
		Api:           model.APIOpenAICompletions,
		Provider:      provider,
		BaseURL:       "https://example.test/v1",
		Input:         []string{"text"},
		ContextWindow: 1000,
		MaxTokens:     100,
	}
}

func TestInMemoryModelsStoreRoundTrip(t *testing.T) {
	store := NewInMemoryCodingAgentModelsStore()
	ctx := context.Background()
	if got, err := store.Read(ctx, "one"); err != nil || got != nil {
		t.Fatalf("read empty = %+v, %v", got, err)
	}
	entry := ModelsStoreEntry{Models: []*model.Model{testModel("one", "m1")}, CheckedAt: 100}
	if err := store.Write(ctx, "one", entry); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.Models) != 1 || got.Models[0].ID != "m1" || got.CheckedAt != 100 {
		t.Fatalf("read = %+v", got)
	}
	if got.Models[0] == entry.Models[0] {
		t.Fatal("store should clone models")
	}
	if err := store.Delete(ctx, "one"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Read(ctx, "one"); err != nil || got != nil {
		t.Fatalf("read after delete = %+v, %v", got, err)
	}
}

func TestFileModelsStorePersistsProviders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models-store.json")
	store := NewFileModelsStore(path)
	ctx := context.Background()
	if err := store.Write(ctx, "one", ModelsStoreEntry{Models: []*model.Model{testModel("one", "m1")}, CheckedAt: 100}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, "two", ModelsStoreEntry{Models: []*model.Model{testModel("two", "m2")}, CheckedAt: 200}); err != nil {
		t.Fatal(err)
	}

	reloaded := NewFileModelsStore(path)
	one, err := reloaded.Read(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	if one == nil || len(one.Models) != 1 || one.Models[0].ID != "m1" || one.CheckedAt != 100 {
		t.Fatalf("one = %+v", one)
	}
	two, err := reloaded.Read(ctx, "two")
	if err != nil {
		t.Fatal(err)
	}
	if two == nil || two.Models[0].ID != "m2" {
		t.Fatalf("two = %+v", two)
	}

	if err := reloaded.Delete(ctx, "one"); err != nil {
		t.Fatal(err)
	}
	if got, err := reloaded.Read(ctx, "one"); err != nil || got != nil {
		t.Fatalf("one after delete = %+v, %v", got, err)
	}
	if got, _ := reloaded.Read(ctx, "two"); got == nil || got.Models[0].ID != "m2" {
		t.Fatalf("two after delete = %+v", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("models store file is empty")
	}
}

func TestFileModelsStoreHonorsCancellation(t *testing.T) {
	store := NewInMemoryCodingAgentModelsStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Read(ctx, "one"); err == nil {
		t.Fatal("expected cancellation error")
	}
}
