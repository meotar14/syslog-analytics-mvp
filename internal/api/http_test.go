package api

import (
	"strings"
	"testing"
)

func TestIndexContainsLoadingShell(t *testing.T) {
	body, err := assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read embedded index.html: %v", err)
	}
	html := string(body)

	wants := []string{
		"id=\"loadingIndicator\"",
		"id=\"loadingStatus\"",
		"data-loading-group=\"overview\"",
		"data-loading-group=\"trend\"",
		"data-loading-group=\"sources\"",
		"data-loading-group=\"severity\"",
		"data-loading-group=\"facility\"",
		"data-loading-group=\"settings\"",
		"function setLoading(label = \"Loading...\")",
		"function clearLoading(message = \"Updated\", type = \"success\")",
		"function failLoading(message)",
		"async function withLoading(label, action, doneLabel = \"Done\")",
		"async function loadSection(group, label, action)",
		"Refreshing overview...",
		"Loading overview…",
		"Loading sources…",
		"Loading settings…",
	}
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Fatalf("index.html missing %q", want)
		}
	}
}
