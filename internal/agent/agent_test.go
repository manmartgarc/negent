package agent

import (
	"testing"
)

func TestAllCategories(t *testing.T) {
	cats := AllCategories()
	if len(cats) != 6 {
		t.Errorf("AllCategories() returned %d categories, want 6", len(cats))
	}

	expected := map[Category]bool{
		CategoryConfig:     true,
		CategoryCustomCode: true,
		CategoryMemory:     true,
		CategorySessions:   true,
		CategoryHistory:    true,
		CategoryPlugins:    true,
	}
	for _, c := range cats {
		if !expected[c] {
			t.Errorf("unexpected category: %q", c)
		}
	}
}

func TestCategoryStringValues(t *testing.T) {
	tests := []struct {
		cat  Category
		want string
	}{
		{CategoryConfig, "config"},
		{CategoryCustomCode, "custom-code"},
		{CategoryMemory, "memory"},
		{CategorySessions, "sessions"},
		{CategoryHistory, "history"},
		{CategoryPlugins, "plugins"},
	}
	for _, tt := range tests {
		if string(tt.cat) != tt.want {
			t.Errorf("Category %v = %q, want %q", tt.cat, string(tt.cat), tt.want)
		}
	}
}
