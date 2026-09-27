package store

import "testing"

func TestAddRecentMovesToTopAndCaps(t *testing.T) {
	var s State
	for i := 0; i < 10; i++ {
		s.AddRecent(Recent{AccountID: string(rune('a' + i)), Role: "Dev"})
	}
	if len(s.Recents) != maxRecents {
		t.Fatalf("len = %d, want %d", len(s.Recents), maxRecents)
	}
	s.AddRecent(Recent{AccountID: "e", Role: "Dev"})
	if s.Recents[0].AccountID != "e" {
		t.Errorf("most recent should be first, got %q", s.Recents[0].AccountID)
	}
	seen := 0
	for _, r := range s.Recents {
		if r.AccountID == "e" {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("duplicate recent entries: %d", seen)
	}
}

func TestToggleFavorite(t *testing.T) {
	var s State
	s.ToggleFavorite("1")
	s.ToggleFavorite("2")
	s.ToggleFavorite("1")
	if len(s.Favorites) != 1 || s.Favorites[0] != "2" {
		t.Errorf("favorites = %v", s.Favorites)
	}
}
