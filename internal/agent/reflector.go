package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ctfcode/internal/event"
)

// ReflectorFailureMode describes the type of failure the reflector detects.
type ReflectorFailureMode string

const (
	FailureToolError  ReflectorFailureMode = "tool_error"
	FailureEmptyTurn  ReflectorFailureMode = "empty_turn"
	FailurePlanStuck  ReflectorFailureMode = "plan_stuck"
	FailureLoop       ReflectorFailureMode = "loop"
)

// Reflector intervenes when the agent experiences repeated failures.
// Inspired by PentAGI's Reflector integration, it analyzes failure patterns,
// identifies root causes, and generates corrective guidance to get the
// agent unstuck and back on track.
type Reflector struct {
	failureCount       int
	consecutiveFailures int
	lastFailurePattern ReflectorFailureMode
	lastFailureTime    time.Time

	// Configuration
	MaxConsecutiveFailures int // consecutive failures before reflector intervenes (default: 3)
	CooldownSeconds        int // seconds between reflector interventions (default: 30)
}

// NewReflector creates a new reflector with default thresholds.
func NewReflector() *Reflector {
	return &Reflector{
		MaxConsecutiveFailures: 3,
		CooldownSeconds:        30,
	}
}

// NewReflectorWithLimits creates a reflector with custom thresholds.
func NewReflectorWithLimits(maxFailures, cooldownSecs int) *Reflector {
	if maxFailures <= 0 {
		maxFailures = 3
	}
	if cooldownSecs <= 0 {
		cooldownSecs = 30
	}
	return &Reflector{
		MaxConsecutiveFailures: maxFailures,
		CooldownSeconds:        cooldownSecs,
	}
}

// RecordToolResult records a tool execution result and returns a reflector
// assessment if intervention is needed. Returns nil when everything is fine.
func (r *Reflector) RecordToolResult(ctx context.Context, toolName string, errStr string, output string) *event.ReflectorResult {
	// Check cooldown period
	if !r.lastFailureTime.IsZero() && time.Since(r.lastFailureTime).Seconds() < float64(r.CooldownSeconds) {
		return nil // still in cooldown
	}

	if errStr != "" || output == "" || isStuckOutput(output) {
		r.consecutiveFailures++
	} else {
		r.consecutiveFailures = 0
		return nil // success, no intervention needed
	}

	if r.consecutiveFailures < r.MaxConsecutiveFailures {
		return nil // not enough consecutive failures to warrant intervention
	}

	return r.analyzeAndIntervene(ctx, toolName, errStr)
}

// RecordEmptyTurn records an empty turn (model produced no tool calls and no
// visible final answer) and returns reflector assessment if needed.
func (r *Reflector) RecordEmptyTurn(ctx context.Context) *event.ReflectorResult {
	if !r.lastFailureTime.IsZero() && time.Since(r.lastFailureTime).Seconds() < float64(r.CooldownSeconds) {
		return nil
	}

	r.consecutiveFailures++
	r.failureCount++

	if r.consecutiveFailures < r.MaxConsecutiveFailures {
		return nil
	}

	start := time.Now()
	pattern := FailureEmptyTurn
	r.lastFailurePattern = pattern
	r.lastFailureTime = time.Now()
	rr := &event.ReflectorResult{
		FailureCount:   r.failureCount,
		FailurePattern: string(pattern),
		RootCause:      "Model produced empty response without tool calls or final answer",
		CorrectiveAction: "Provide clearer instructions or check if the model has sufficient context",
		Guidance: fmt.Sprintf(
			"[Reflector] The model has produced empty responses %d consecutive times. "+
				"This may indicate the model lacks sufficient context or the prompt is ambiguous. "+
				"Consider providing more specific guidance about what tools to use and what outputs are expected.",
			r.consecutiveFailures,
		),
	}
	rr.DurationMs = time.Since(start).Milliseconds()
	return rr
}

// analyzeAndIntervene performs root cause analysis and generates guidance.
func (r *Reflector) analyzeAndIntervene(ctx context.Context, toolName string, errStr string) *event.ReflectorResult {
	start := time.Now()
	r.failureCount++

	rr := &event.ReflectorResult{
		FailureCount: r.failureCount,
	}

	// Classify the failure pattern
	errStr = strings.ToLower(errStr)
	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "timed out"):
		rr.FailurePattern = string(FailureToolError)
		rr.RootCause = "Tool execution timed out — the operation may be too slow or the target is unresponsive"
		rr.CorrectiveAction = "Increase timeout, check target availability, or try a faster approach"
	case strings.Contains(errStr, "denied") || strings.Contains(errStr, "permission") || strings.Contains(errStr, "forbidden"):
		rr.FailurePattern = string(FailureToolError)
		rr.RootCause = "Permission denied — the agent lacks the necessary access rights"
		rr.CorrectiveAction = "Verify credentials and permissions, or request elevated access"
	case strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such"):
		rr.FailurePattern = string(FailureToolError)
		rr.RootCause = "Target resource not found — the path or identifier may be incorrect"
		rr.CorrectiveAction = "Verify the target path/identifier exists and is accessible"
	case strings.Contains(errStr, "connection") || strings.Contains(errStr, "refused") || strings.Contains(errStr, "unreachable"):
		rr.FailurePattern = string(FailureToolError)
		rr.RootCause = "Network connection failed — the target service may be down or unreachable"
		rr.CorrectiveAction = "Check network connectivity and target service status"
	default:
		rr.FailurePattern = string(FailureToolError)
		rr.RootCause = fmt.Sprintf("Tool %q failed with an error", toolName)
		rr.CorrectiveAction = "Review the error details and adjust the approach"
	}

	rr.Guidance = fmt.Sprintf(
		"[Reflector] Tool %q has failed %d consecutive times. "+
			"Root cause: %s. Recommended action: %s. "+
			"Consider an alternative approach rather than continuing to retry the same failing operation.",
		toolName, r.consecutiveFailures, rr.RootCause, rr.CorrectiveAction,
	)

	r.lastFailurePattern = ReflectorFailureMode(rr.FailurePattern)
	r.lastFailureTime = time.Now()

	rr.DurationMs = time.Since(start).Milliseconds()
	return rr
}

// Reset clears the reflector state for a new turn or after successful recovery.
func (r *Reflector) Reset() {
	r.failureCount = 0
	r.consecutiveFailures = 0
	r.lastFailurePattern = ""
	r.lastFailureTime = time.Time{}
}

// isStuckOutput detects when tool output indicates the agent is stuck.
func isStuckOutput(output string) bool {
	output = strings.TrimSpace(output)
	if output == "" {
		return true
	}
	stuckSignals := []string{
		"command not found",
		"no route to host",
		"operation timed out",
		"connection refused",
	}
	outputLower := strings.ToLower(output)
	for _, signal := range stuckSignals {
		if strings.Contains(outputLower, signal) {
			return true
		}
	}
	return false
}
