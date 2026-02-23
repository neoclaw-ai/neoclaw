package approval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/machinae/betterclaw/internal/store"
)

// User is one authorized user record in allowed_users.json.
type User struct {
	ID       string    `json:"id"`
	Channel  string    `json:"channel"`
	Username string    `json:"username"`
	Name     string    `json:"name"`
	AddedAt  time.Time `json:"added_at"`
}

// UsersFile is the on-disk shape for the allowed users store.
type UsersFile struct {
	Users []User `json:"users"`
}

// LoadUsers loads allowed users from path. Missing files return an empty list.
func LoadUsers(path string) (UsersFile, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return UsersFile{}, errors.New("allowed users path is required")
	}

	content, err := store.ReadFile(trimmedPath)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return UsersFile{Users: []User{}}, nil
	default:
		return UsersFile{}, fmt.Errorf("read allowed users file %q: %w", trimmedPath, err)
	}

	if len(strings.TrimSpace(content)) == 0 {
		return UsersFile{Users: []User{}}, nil
	}

	var usersFile UsersFile
	if err := json.Unmarshal([]byte(content), &usersFile); err != nil {
		return UsersFile{}, fmt.Errorf("decode allowed users file %q: %w", trimmedPath, err)
	}
	if usersFile.Users == nil {
		usersFile.Users = []User{}
	}
	return usersFile, nil
}

func saveUsers(path string, usersFile UsersFile) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return errors.New("allowed users path is required")
	}
	if usersFile.Users == nil {
		usersFile.Users = []User{}
	}

	encoded, err := json.MarshalIndent(usersFile, "", "  ")
	if err != nil {
		return fmt.Errorf("encode allowed users: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := store.WriteFile(trimmedPath, encoded); err != nil {
		return fmt.Errorf("write allowed users file %q: %w", trimmedPath, err)
	}
	return nil
}

// IsAllowedUser reports whether a user ID is authorized for one channel.
func IsAllowedUser(usersFile UsersFile, id, channel string) bool {
	targetID := strings.TrimSpace(id)
	targetChannel := strings.ToLower(strings.TrimSpace(channel))
	if targetID == "" || targetChannel == "" {
		return false
	}

	for _, user := range usersFile.Users {
		if strings.TrimSpace(user.ID) == targetID && strings.ToLower(strings.TrimSpace(user.Channel)) == targetChannel {
			return true
		}
	}
	return false
}

// AddUser adds or updates one id+channel user entry and atomically writes the file.
func AddUser(path string, user User) error {
	targetID := strings.TrimSpace(user.ID)
	targetChannel := strings.ToLower(strings.TrimSpace(user.Channel))
	if targetID == "" {
		return errors.New("user id is required")
	}
	if targetChannel == "" {
		return errors.New("channel is required")
	}
	username := strings.TrimSpace(user.Username)
	name := strings.TrimSpace(user.Name)

	usersFile, err := loadCachedUsersFile(path)
	if err != nil {
		return err
	}

	for i := range usersFile.Users {
		if strings.TrimSpace(usersFile.Users[i].ID) == targetID &&
			strings.ToLower(strings.TrimSpace(usersFile.Users[i].Channel)) == targetChannel {
			usersFile.Users[i].Username = username
			usersFile.Users[i].Name = name
			return saveCachedUsersFile(path, usersFile)
		}
	}

	addedAt := user.AddedAt.UTC()
	if addedAt.IsZero() {
		addedAt = time.Now().UTC()
	}

	usersFile.Users = append(usersFile.Users, User{
		ID:       targetID,
		Channel:  targetChannel,
		Username: username,
		Name:     name,
		AddedAt:  addedAt,
	})
	return saveCachedUsersFile(path, usersFile)
}
