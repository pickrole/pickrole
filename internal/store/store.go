// Package store keeps PickRole's local state: the cached account list, the
// recently used profiles and the favourites.
package store

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pickrole/pickrole/internal/fsutil"
)

// Account is an AWS account with the roles the user can assume in it.
type Account struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// Snapshot is the cached account list.
type Snapshot struct {
	StartURL  string    `json:"startUrl"`
	UpdatedAt time.Time `json:"updatedAt"`
	Accounts  []Account `json:"accounts"`
}

// Recent is a profile the user loaded before.
type Recent struct {
	AccountID   string    `json:"accountId"`
	AccountName string    `json:"accountName"`
	Role        string    `json:"role"`
	UsedAt      time.Time `json:"usedAt"`
}

// Active describes the profile currently written to disk.
type Active struct {
	AccountID             string    `json:"accountId"`
	AccountName           string    `json:"accountName"`
	Role                  string    `json:"role"`
	Profile               string    `json:"profile"`
	ExpiresAt             time.Time `json:"expiresAt"`
	CodeArtifact          bool      `json:"codeArtifact"`
	CodeArtifactExpiresAt time.Time `json:"codeArtifactExpiresAt"`
}

// State is everything PickRole remembers between runs besides the config.
type State struct {
	Favorites []string `json:"favorites"`
	Recents   []Recent `json:"recents"`
	Active    *Active  `json:"active,omitempty"`
	// CodeArtifactAccess remembers, per "accountID/role", whether the role
	// could get a CodeArtifact token the last time it was loaded.
	CodeArtifactAccess map[string]bool `json:"codeArtifactAccess,omitempty"`
}

// AccessKey is the key used in CodeArtifactAccess.
func AccessKey(accountID, role string) string { return accountID + "/" + role }

// SetCodeArtifactAccess records whether accountID/role has CodeArtifact.
func (s *State) SetCodeArtifactAccess(accountID, role string, ok bool) {
	if s.CodeArtifactAccess == nil {
		s.CodeArtifactAccess = map[string]bool{}
	}
	s.CodeArtifactAccess[AccessKey(accountID, role)] = ok
}

const maxRecents = 8

func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pickrole"), nil
}

func stateDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pickrole"), nil
}

// LoadSnapshot returns the cached account list, if any.
func LoadSnapshot() (Snapshot, bool, error) {
	dir, err := cacheDir()
	if err != nil {
		return Snapshot{}, false, err
	}
	var s Snapshot
	ok, err := fsutil.ReadJSON(filepath.Join(dir, "accounts.json"), &s)
	return s, ok, err
}

// SaveSnapshot caches the account list, sorted by name.
func SaveSnapshot(s Snapshot) error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	sort.Slice(s.Accounts, func(i, j int) bool { return s.Accounts[i].Name < s.Accounts[j].Name })
	for i := range s.Accounts {
		sort.Strings(s.Accounts[i].Roles)
	}
	return fsutil.WriteJSON(filepath.Join(dir, "accounts.json"), s, 0o600)
}

// LoadState reads favourites, recents and the active profile.
func LoadState() (State, error) {
	dir, err := stateDir()
	if err != nil {
		return State{}, err
	}
	var s State
	_, err = fsutil.ReadJSON(filepath.Join(dir, "state.json"), &s)
	return s, err
}

// SaveState writes favourites, recents and the active profile.
func SaveState(s State) error {
	dir, err := stateDir()
	if err != nil {
		return err
	}
	return fsutil.WriteJSON(filepath.Join(dir, "state.json"), s, 0o600)
}

// AddRecent puts r at the top of the recents, removing an older copy.
func (s *State) AddRecent(r Recent) {
	out := []Recent{r}
	for _, old := range s.Recents {
		if old.AccountID == r.AccountID && old.Role == r.Role {
			continue
		}
		out = append(out, old)
	}
	if len(out) > maxRecents {
		out = out[:maxRecents]
	}
	s.Recents = out
}

// ToggleFavorite adds or removes an account from the favourites.
func (s *State) ToggleFavorite(accountID string) {
	for i, id := range s.Favorites {
		if id == accountID {
			s.Favorites = append(s.Favorites[:i], s.Favorites[i+1:]...)
			return
		}
	}
	s.Favorites = append(s.Favorites, accountID)
}
