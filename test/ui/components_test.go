package ui_test

import (
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components"
)

// stripANSI removes ANSI escape sequences so assertions can target the
// rendered text content without coupling to specific style codes.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		if r == 0x1b {
			inEscape = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestRenderTilePlainNumber(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := stripANSI(components.RenderTile(th, 40, "employee_total", "56", components.TileFormat{}))

	if !strings.Contains(out, "EMPLOYEE TOTAL") {
		t.Fatalf("expected uppercased label, got %q", out)
	}
	if !strings.Contains(out, "56") {
		t.Fatalf("expected value 56 in tile, got %q", out)
	}
}

func TestRenderTileCurrencyFormatting(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := stripANSI(components.RenderTile(th, 60, "revenue", "12345.6", components.TileFormat{
		Kind: "currency",
	}))

	if !strings.Contains(out, "$12,345.60") {
		t.Fatalf("expected $12,345.60, got %q", out)
	}
}

func TestRenderTilePercentFormatting(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := stripANSI(components.RenderTile(th, 40, "growth", "12.5", components.TileFormat{
		Kind: "percent",
	}))

	if !strings.Contains(out, "12.5%") {
		t.Fatalf("expected 12.5%%, got %q", out)
	}
}

func TestRenderTileBool(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := stripANSI(components.RenderTile(th, 40, "is_active", "true", components.TileFormat{
		Kind: "bool",
	}))

	if !strings.Contains(out, "Yes") {
		t.Fatalf("expected Yes, got %q", out)
	}
}

func TestRenderTileAutoThousandsSeparator(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := stripANSI(components.RenderTile(th, 40, "count", "1234567", components.TileFormat{}))

	if !strings.Contains(out, "1,234,567") {
		t.Fatalf("expected thousands separator, got %q", out)
	}
}

func TestChartBadgeUnsupportedTypes(t *testing.T) {
	t.Parallel()

	th := theme.Default()

	for _, kind := range []string{"bar", "line", "pie", "scatter", "area", "radar"} {
		out := stripANSI(components.ChartBadge(th, kind))
		if !strings.Contains(out, "[chart: "+kind+"]") {
			t.Fatalf("expected badge for %q, got %q", kind, out)
		}
	}
}

func TestChartBadgeOmittedForRenderedTypes(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	for _, kind := range []string{"", "table", "tile", "TABLE", " Tile "} {
		if got := components.ChartBadge(th, kind); got != "" {
			t.Fatalf("expected empty badge for %q, got %q", kind, got)
		}
	}
}

func TestTileFormatFromConfig(t *testing.T) {
	t.Parallel()

	cfg := map[string]any{
		"format":   "currency",
		"currency": "€",
		"decimals": float64(0),
	}
	f := components.TileFormatFromConfig(cfg)
	if f.Kind != "currency" || f.CurrencySymbol != "€" || f.Decimals != 0 || !f.DecimalsSet {
		t.Fatalf("unexpected format: %+v", f)
	}
}
