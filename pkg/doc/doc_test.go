package doc

import (
	"testing"

	"github.com/go-go-golems/glazed/pkg/help"
)

func TestAddDocToHelpSystemLoadsExpectedSections(t *testing.T) {
	hs := help.NewHelpSystem()
	if err := AddDocToHelpSystem(hs); err != nil {
		t.Fatalf("AddDocToHelpSystem: %v", err)
	}

	expectedSlugs := []string{
		"codex-session-getting-started",
		"codex-session-reference-examples",
		"codex-session-architecture",
	}

	for _, slug := range expectedSlugs {
		if _, err := hs.GetSectionWithSlug(slug); err != nil {
			t.Fatalf("missing help slug %q: %v", slug, err)
		}
	}
}
