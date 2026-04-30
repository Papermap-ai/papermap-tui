package app

import "testing"

func TestScrubChildEnvDropsPapermapPrefix(t *testing.T) {
	t.Parallel()
	in := []string{
		"HOME=/u/me",
		"PAPERMAP_API_URL=https://api.example",
		"PATH=/usr/bin",
		"PAPERMAP_TOKEN=secret",
		"USER=me",
	}
	got := scrubChildEnv(in)
	for _, kv := range got {
		if len(kv) >= len("PAPERMAP_") && kv[:len("PAPERMAP_")] == "PAPERMAP_" {
			t.Fatalf("PAPERMAP_ leaked: %q", kv)
		}
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3: %v", len(got), got)
	}
}

func TestScrubChildEnvNilSafe(t *testing.T) {
	t.Parallel()
	got := scrubChildEnv(nil)
	if len(got) != 0 {
		t.Fatalf("nil in -> non-empty out: %v", got)
	}
}
