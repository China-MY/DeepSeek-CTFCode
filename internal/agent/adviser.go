package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ctfcode/internal/event"
)

// Adviser monitors the main agent's tool-call patterns during execution.
// Inspired by PentAGI's Execution Monitoring system, it detects loops,
// repeated errors, and inefficient patterns, then provides corrective
// guidance to keep the agent on track.
type Adviser struct {
	sameToolCount    int    // consecutive calls to the same tool
	sameToolName     string // the tool being repeated
	totalToolCalls   int    // total tool calls this turn
	stuckOnError     bool   // true when the same error repeats
	lastErrorContent string // content of the last error for dedup

	// Configuration
	SameToolLimit int // consecutive same-tool calls before flagging (default: 3)
	TotalLimit    int // total tool calls before flagging (default: 25)
}

// NewAdviser creates a new execution monitor with default thresholds.
func NewAdviser() *Adviser {
	return &Adviser{
		SameToolLimit: 3,
		TotalLimit:    25,
	}
}

// NewAdviserWithLimits creates an adviser with custom thresholds.
func NewAdviserWithLimits(sameToolLimit, totalLimit int) *Adviser {
	if sameToolLimit <= 0 {
		sameToolLimit = 3
	}
	if totalLimit <= 0 {
		totalLimit = 25
	}
	return &Adviser{
		SameToolLimit: sameToolLimit,
		TotalLimit:    totalLimit,
	}
}

// RecordToolCall records a tool call and returns guidance if a problematic
// pattern is detected. Returns empty string when everything looks fine.
func (a *Adviser) RecordToolCall(ctx context.Context, toolName string, toolResult string, errStr string) *event.AdviserResult {
	start := time.Now()
	a.totalToolCalls++

	ar := &event.AdviserResult{
		TotalToolCalls: a.totalToolCalls,
	}

	// Track same-tool repetition
	if toolName == a.sameToolName {
		a.sameToolCount++
	} else {
		a.sameToolCount = 1
		a.sameToolName = toolName
	}
	ar.SameToolCount = a.sameToolCount
	ar.SameToolName = a.sameToolName

	// Check for tool error repetition
	if errStr != "" {
		errorSnippet := truncateString(errStr, 120)
		if a.lastErrorContent != "" && strings.Contains(errorSnippet, truncateString(a.lastErrorContent, 60)) {
			a.stuckOnError = true
			ar.StuckOnError = true
			ar.LastErrorFragment = errorSnippet
		}
		a.lastErrorContent = errStr
	} else {
		a.stuckOnError = false
		a.lastErrorContent = ""
	}

	// Loop detection: same tool called N+ times in a row
	if a.sameToolCount >= a.SameToolLimit {
		ar.IsLoop = true
		ar.Guidance = fmt.Sprintf(
			"[Adviser] Loop detected: you have called %q %d consecutive times. "+
				"If the previous attempts did not produce the desired result, try a different approach, "+
				"tool, or target instead of repeating the same action.",
			a.sameToolName, a.sameToolCount,
		)
	}

	// Stuck-on-error detection
	if a.stuckOnError && ar.Guidance == "" {
		ar.Guidance = fmt.Sprintf(
			"[Adviser] The same error pattern is repeating: %q. "+
				"Consider diagnosing the root cause first (check service status, permissions, "+
				"or input validity) before retrying.",
			truncateString(a.lastErrorContent, 80),
		)
		ar.IsLoop = true
	}

	// Total call threshold warning
	if a.totalToolCalls >= a.TotalLimit && ar.Guidance == "" {
		ar.Guidance = fmt.Sprintf(
			"[Adviser] You have made %d tool calls this turn. "+
				"If progress has stalled, consider summarizing what you have accomplished and "+
				"proposing a revised plan rather than continuing to call tools indiscriminately.",
			a.totalToolCalls,
		)
	}

	ar.DurationMs = time.Since(start).Milliseconds()
	return ar
}

// Reset clears the adviser's state for a new turn.
func (a *Adviser) Reset() {
	a.sameToolCount = 0
	a.sameToolName = ""
	a.totalToolCalls = 0
	a.stuckOnError = false
	a.lastErrorContent = ""
}

// truncateString truncates a string to maxLen runes.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
