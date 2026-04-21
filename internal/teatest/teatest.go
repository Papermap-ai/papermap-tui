// Package teatest provides shared helpers for testing Bubble Tea models.
//
// Bubble Tea commands frequently return tea.BatchMsg — a slice of follow-up
// commands — when a single update produces multiple side effects (a domain
// message plus a spinner tick, for example). Tests usually only care about
// one of those messages. FindMsg walks a command's output recursively and
// returns the first message of the requested type.
package teatest

import (
	tea "charm.land/bubbletea/v2"
)

// FindMsg runs cmd and walks any returned tea.BatchMsg recursively, returning
// the first message of type T it encounters. It returns the zero value and
// false when cmd is nil or no message of type T is produced.
//
// Use it in tests to extract a specific domain message from a model's update
// or init output without caring about ordering or other batched commands.
func FindMsg[T tea.Msg](cmd tea.Cmd) (T, bool) {
	var zero T
	if cmd == nil {
		return zero, false
	}
	return findInMsg[T](cmd())
}

func findInMsg[T tea.Msg](msg tea.Msg) (T, bool) {
	var zero T
	switch v := msg.(type) {
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			if got, ok := findInMsg[T](c()); ok {
				return got, true
			}
		}
	case T:
		return v, true
	}
	return zero, false
}
