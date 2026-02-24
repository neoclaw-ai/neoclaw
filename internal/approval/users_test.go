package approval

import (
	"path/filepath"
	"testing"

	"github.com/neoclaw-ai/neoclaw/internal/store"
)

func TestLoadUsers_MissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowed_users.json")

	usersFile, err := LoadUsers(path)
	if err != nil {
		t.Fatalf("load users: %v", err)
	}
	if len(usersFile.Users) != 0 {
		t.Fatalf("expected empty users for missing file, got %d", len(usersFile.Users))
	}
}

func TestLoadUsers_ParsesOneEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowed_users.json")
	raw := []byte(`{
  "users": [
    {
      "id": "987654321",
      "channel": "telegram",
      "username": "alice",
      "name": "Alice",
      "added_at": "2026-02-19T14:30:00Z"
    }
  ]
}
`)
	if err := store.WriteFile(path, raw); err != nil {
		t.Fatalf("write users file: %v", err)
	}

	usersFile, err := LoadUsers(path)
	if err != nil {
		t.Fatalf("load users: %v", err)
	}
	if len(usersFile.Users) != 1 {
		t.Fatalf("expected one user, got %d", len(usersFile.Users))
	}
	if usersFile.Users[0].ID != "987654321" {
		t.Fatalf("unexpected id: %q", usersFile.Users[0].ID)
	}
	if usersFile.Users[0].Channel != "telegram" {
		t.Fatalf("unexpected channel: %q", usersFile.Users[0].Channel)
	}
}

func TestIsAllowedUser_MatchesIDAndChannel(t *testing.T) {
	usersFile := UsersFile{
		Users: []User{
			{ID: "42", Channel: "telegram"},
		},
	}

	if !IsAllowedUser(usersFile, "42", "telegram") {
		t.Fatalf("expected user 42 telegram to be allowed")
	}
	if IsAllowedUser(usersFile, "43", "telegram") {
		t.Fatalf("expected unknown id to be denied")
	}
	if IsAllowedUser(usersFile, "42", "cli") {
		t.Fatalf("expected known id on different channel to be denied")
	}
}

func TestAddUser_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowed_users.json")
	if err := AddUser(path, User{
		ID:       "123",
		Channel:  "telegram",
		Username: "alice",
		Name:     "Alice",
	}); err != nil {
		t.Fatalf("add user: %v", err)
	}

	loaded, err := LoadUsers(path)
	if err != nil {
		t.Fatalf("reload users: %v", err)
	}
	if len(loaded.Users) != 1 {
		t.Fatalf("expected one user after round trip, got %d", len(loaded.Users))
	}
	if loaded.Users[0].ID != "123" {
		t.Fatalf("unexpected id after round trip: %q", loaded.Users[0].ID)
	}
	if loaded.Users[0].Channel != "telegram" {
		t.Fatalf("unexpected channel after round trip: %q", loaded.Users[0].Channel)
	}
	if loaded.Users[0].Username != "alice" {
		t.Fatalf("unexpected username after round trip: %q", loaded.Users[0].Username)
	}
	if loaded.Users[0].Name != "Alice" {
		t.Fatalf("unexpected name after round trip: %q", loaded.Users[0].Name)
	}
	if loaded.Users[0].AddedAt.IsZero() {
		t.Fatal("expected added_at to be set")
	}
}

func TestAddUser_ExistingUserUpdatesMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowed_users.json")
	if err := AddUser(path, User{
		ID:       "123",
		Channel:  "telegram",
		Username: "alice",
		Name:     "Alice",
	}); err != nil {
		t.Fatalf("add user first time: %v", err)
	}
	if err := AddUser(path, User{
		ID:       "123",
		Channel:  "telegram",
		Username: "alice2",
		Name:     "Alice Updated",
	}); err != nil {
		t.Fatalf("add user second time: %v", err)
	}

	loaded, err := LoadUsers(path)
	if err != nil {
		t.Fatalf("reload users: %v", err)
	}
	if len(loaded.Users) != 1 {
		t.Fatalf("expected one user after update, got %d", len(loaded.Users))
	}
	if loaded.Users[0].Username != "alice2" {
		t.Fatalf("unexpected updated username: %q", loaded.Users[0].Username)
	}
	if loaded.Users[0].Name != "Alice Updated" {
		t.Fatalf("unexpected updated name: %q", loaded.Users[0].Name)
	}
}
