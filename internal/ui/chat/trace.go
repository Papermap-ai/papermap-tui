package chat

import (
	"fmt"
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
)

// TraceKind identifies the category of a single TraceStep so the renderer
// can style each entry consistently.
type TraceKind int

const (
	// TraceThought captures Alan's reasoning narrative, accumulated from
	// agent_thought / reasoning SSE deltas.
	TraceThought TraceKind = iota
	// TraceToolCall represents a tool invocation announced by the agent
	// (e.g. a SQL query). Body usually carries the resolved arguments.
	TraceToolCall
	// TraceToolOutput attaches the tool's result preview to its matching
	// TraceToolCall step. It does not appear as a top-level entry; the
	// renderer folds it under the originating call.
	TraceToolOutput
)

// TraceStep is a single entry in an assistant message's reasoning trace.
// The chat layer assembles these from API stream events so the UI can show
// what Alan is doing while waiting for the final answer.
type TraceStep struct {
	Kind       TraceKind
	ToolCallID string
	// Title is the short label shown above the body (e.g. tool display
	// name, "Thinking"). Optional; falls back to a kind-based default.
	Title string
	// Body is the long-form content (thought text, SQL, JSON args).
	Body string
	// Output is the result preview attached after a paired tool_output /
	// tool_call_complete arrives. Optional.
	Output string
	// DurationMS captures the tool execution duration when the backend
	// reports it on tool_call_complete.
	DurationMS float64
	// Status is the tool's terminal status ("success", "error", ...).
	Status string
	// Iteration records the agent loop iteration that produced this step.
	// Used to coalesce streamed thought deltas into a single Thought
	// entry per iteration.
	Iteration int
}

// MergeThoughtDelta appends delta to the most recent Thought step in steps
// for the given iteration. If no matching open step exists, a new one is
// appended. Returns the (possibly updated) slice. complete signals the
// final delta of the thought block; subsequent calls with the same
// iteration after completion start a new step.
func MergeThoughtDelta(steps []TraceStep, iteration int, delta string, complete bool) []TraceStep {
	if delta == "" && !complete {
		return steps
	}

	// Look for the trailing Thought step of the same iteration that has
	// not yet been marked complete. Marker: empty Status. We use Status
	// == "complete" to seal a thought block.
	idx := -1
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Kind != TraceThought {
			continue
		}
		// Tool steps in between are fine; we still keep merging the
		// open thought block as the source of truth for that iteration.
		if steps[i].Iteration == iteration && steps[i].Status != "complete" {
			idx = i
			break
		}
		break
	}

	if idx == -1 {
		step := TraceStep{
			Kind:      TraceThought,
			Title:     "Thinking",
			Body:      delta,
			Iteration: iteration,
		}
		if complete {
			step.Status = "complete"
		}
		return append(steps, step)
	}

	steps[idx].Body += delta
	if complete {
		steps[idx].Status = "complete"
	}
	return steps
}

// AttachToolOutput finds the most recent TraceToolCall matching toolCallID
// (or the most recent unfinished call when toolCallID is empty) and folds
// the output / status into it. Returns the (possibly updated) slice.
func AttachToolOutput(steps []TraceStep, toolCallID string, output string, status string, durationMS float64) []TraceStep {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Kind != TraceToolCall {
			continue
		}
		if toolCallID != "" && steps[i].ToolCallID != toolCallID {
			continue
		}
		if output != "" {
			if steps[i].Output != "" {
				steps[i].Output += "\n" + output
			} else {
				steps[i].Output = output
			}
		}
		if status != "" {
			steps[i].Status = status
		}
		if durationMS > 0 {
			steps[i].DurationMS = durationMS
		}
		return steps
	}
	return steps
}

// AppendToolOutputContent appends streamed tool_output content to the
// matching tool call. Used for the streaming text path so the UI can show
// in-flight output before tool_call_complete arrives.
func AppendToolOutputContent(steps []TraceStep, toolCallID string, content string) []TraceStep {
	if content == "" {
		return steps
	}
	return AttachToolOutput(steps, toolCallID, content, "", 0)
}

// renderTrace returns the trace block for a message. Returns "" when the
// message has no trace activity to display, or when the user has hidden
// thinking and the trace has finished.
//
// Branches:
//   - showThinking && complete   -> full timeline.
//   - showThinking && streaming  -> live rolling ticker (last 3 steps).
//   - !showThinking && streaming -> muted single-line ellipsis preview.
//   - !showThinking && complete  -> hidden ("").
func renderTrace(th theme.Theme, width int, message Message, showThinking bool) string {
	if len(message.Trace) == 0 {
		return ""
	}

	if !showThinking {
		if message.TraceComplete {
			return ""
		}
		return mutedThinkingPreview(th, message.Trace)
	}

	muted := th.Muted
	accent := th.Accent.Bold(true)

	stepCount := len(message.Trace)
	stepsLabel := fmt.Sprintf("%d step%s", stepCount, pluralS(stepCount))

	var header string
	if message.TraceComplete {
		header = accent.Render(fmt.Sprintf("▾ Thinking · %s", stepsLabel))
	} else {
		// Live: spinner-like header without the toggle hint.
		header = accent.Render("▾ Thinking…")
	}

	var visible []TraceStep
	hidden := 0

	if message.TraceComplete {
		visible = message.Trace
	} else {
		// Live mode: rolling ticker of the last 3 steps.
		const liveWindow = 3
		if len(message.Trace) > liveWindow {
			hidden = len(message.Trace) - liveWindow
			visible = message.Trace[len(message.Trace)-liveWindow:]
		} else {
			visible = message.Trace
		}
	}

	parts := []string{header}
	if hidden > 0 {
		parts = append(parts, muted.Render(fmt.Sprintf("  …and %d earlier step%s", hidden, pluralS(hidden))))
	}
	for _, step := range visible {
		parts = append(parts, renderTraceStep(th, width, step))
	}
	return strings.Join(parts, "\n")
}

// mutedThinkingPreview renders a single-line muted ellipsis preview used
// while a trace is streaming with the thinking toggle off. Shows the
// first 60 runes of the most recent thought (or the latest tool title
// when no thought has streamed yet) so the user retains a faint signal
// of progress without the full reasoning timeline.
func mutedThinkingPreview(th theme.Theme, trace []TraceStep) string {
	snippet := latestThinkingSnippet(trace)
	if snippet == "" {
		return th.Muted.Render("· thinking…")
	}
	return th.Muted.Render("· thinking " + truncateHead(snippet, 60) + "…")
}

// latestThinkingSnippet walks the trace from the end and returns the
// most recent thought body (whitespace-collapsed). Falls back to the
// latest tool call's title when no thought is present, so the preview
// is non-empty as soon as anything starts streaming.
func latestThinkingSnippet(trace []TraceStep) string {
	for i := len(trace) - 1; i >= 0; i-- {
		if trace[i].Kind != TraceThought {
			continue
		}
		body := strings.Join(strings.Fields(trace[i].Body), " ")
		if body != "" {
			return body
		}
	}
	for i := len(trace) - 1; i >= 0; i-- {
		if trace[i].Kind != TraceToolCall {
			continue
		}
		if title := strings.TrimSpace(trace[i].Title); title != "" {
			return title
		}
	}
	return ""
}

// truncateHead returns the first n runes of s. When s already fits, it
// is returned unchanged. Rune-safe so multi-byte characters aren't cut
// mid-codepoint.
func truncateHead(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func renderTraceStep(th theme.Theme, width int, step TraceStep) string {
	switch step.Kind {
	case TraceThought:
		body := strings.TrimSpace(step.Body)
		if body == "" {
			return th.Muted.Render("  · thinking…")
		}
		// Collapse internal whitespace from streaming deltas, then let
		// lipgloss wrap. Italic + faint to feel like peripheral
		// commentary.
		body = strings.Join(strings.Fields(body), " ")
		wrap := max(width-4, 20)
		style := th.Muted.Italic(true).Width(wrap)
		return style.Render("  · " + body)
	case TraceToolCall:
		title := step.Title
		if title == "" {
			title = "Tool call"
		}
		titleLine := th.Accent.Render("  ◆ " + title)

		bodyLines := []string{titleLine}
		if step.Status != "" || step.DurationMS > 0 || step.Output != "" {
			bodyLines = append(bodyLines, renderToolFooter(th, width, step))
		}
		return strings.Join(bodyLines, "\n")
	}
	return ""
}

func renderToolFooter(th theme.Theme, width int, step TraceStep) string {
	meta := []string{}
	if step.Status != "" {
		marker := "→"
		switch strings.ToLower(step.Status) {
		case "success":
			marker = "✓"
		case "error", "failed":
			marker = "✗"
		}
		meta = append(meta, marker+" "+step.Status)
	}
	if step.DurationMS > 0 {
		meta = append(meta, formatDuration(step.DurationMS))
	}

	lines := []string{}
	if len(meta) > 0 {
		lines = append(lines, th.Muted.Render("    "+strings.Join(meta, " · ")))
	}
	if step.Output != "" {
		// Render full output, wrapped. Preserve original line breaks so
		// SQL / code stays readable.
		wrap := max(width-4, 20)
		body := strings.TrimSpace(step.Output)
		style := th.Muted.Width(wrap)
		out := style.Render(body)
		// Indent each rendered line by 4 spaces to align with footer.
		indented := strings.ReplaceAll(out, "\n", "\n    ")
		lines = append(lines, "    "+indented)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func formatDuration(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%.0fms", ms)
	}
	return fmt.Sprintf("%.2fs", ms/1000)
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
