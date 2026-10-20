package scalewaymanager

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// IndexHTML holds exactly the bytes of web/index.html.
func TestIndexHTMLIsEmbedded(t *testing.T) {
	want, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatalf("read web/index.html: %v", err)
	}
	if len(want) == 0 {
		t.Fatal("web/index.html is empty")
	}
	if !bytes.Equal(IndexHTML, want) {
		t.Errorf("IndexHTML (%d bytes) differs from web/index.html (%d bytes)", len(IndexHTML), len(want))
	}
}

// The placeholder page: language, title, viewport, sentence; no script, no inline style.
func TestPlaceholderPage(t *testing.T) {
	page := string(IndexHTML)
	for _, want := range []string{
		`<html lang="en">`,
		`<title>scaleway-manager</title>`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		`scaleway-manager is running; no feature is available yet.`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("web/index.html lacks %q", want)
		}
	}
	lower := strings.ToLower(page)
	for _, banned := range []string{"<script", "<style", "style="} {
		if strings.Contains(lower, banned) {
			t.Errorf("web/index.html contains %q", banned)
		}
	}
}
