// Package charts renders Papermap insight chart payloads as terminal
// strings. Each renderer takes an InsightTable plus a parsed ChartConfig
// and returns a styled string sized to fit the supplied Size.
//
// The package is the single chart-rendering boundary in the TUI. Callers
// invoke charts.Render(chartType, ...) and the registry dispatches to the
// right renderer. New chart types are added by writing a renderer
// function and registering it in registry.go.
//
// Renderers always return a string. On parse or extraction failure they
// return a muted "[chart unavailable: <reason>]" notice rendered via the
// supplied palette so the caller never has to handle errors mid-stream.
//
// All rendering is implemented in pure charm.land/lipgloss/v2. No external
// chart libraries are imported. ntcharts was evaluated and rejected
// because its transitive dependencies pin an older charmbracelet/x/ansi
// API that conflicts with the v2 stack used by this repo.
package charts
