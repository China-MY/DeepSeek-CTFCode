package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ctfcode/internal/event"
)

// TaskPlanner generates structured task plans before execution begins.
// Inspired by PentAGI's Intelligent Task Planning system, it decomposes
// complex tasks into 3-7 actionable steps with risk assessment and
// tool recommendations.
type TaskPlanner struct {
	enabled    bool
	maxSteps   int
}

// NewTaskPlanner creates a new task planner.
func NewTaskPlanner() *TaskPlanner {
	return &TaskPlanner{
		enabled:  true,
		maxSteps: 7,
	}
}

// NewTaskPlannerWithConfig creates a planner with custom settings.
func NewTaskPlannerWithConfig(enabled bool, maxSteps int) *TaskPlanner {
	if maxSteps <= 0 || maxSteps > 15 {
		maxSteps = 7
	}
	return &TaskPlanner{
		enabled:  enabled,
		maxSteps: maxSteps,
	}
}

// IsEnabled reports whether the planner is active.
func (tp *TaskPlanner) IsEnabled() bool { return tp.enabled }

// GeneratePlan creates a structured plan for the given task input.
// Uses heuristic analysis to decompose the task rather than an LLM call
// (keeping it zero-cost for simple tasks).
func (tp *TaskPlanner) GeneratePlan(ctx context.Context, input string) *event.TaskPlanResult {
	if !tp.enabled {
		return nil
	}

	start := time.Now()

	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	plan := &event.TaskPlanResult{
		Strategy: classifyTaskStrategy(input),
	}

	// Decompose the task into steps
	steps := tp.decomposeTask(input)
	plan.Steps = steps
	plan.TotalEstimated = len(steps) * 3 // rough estimate: 3 tool calls per step
	plan.RiskAssessment = assessTaskRisk(input)
	plan.DurationMs = time.Since(start).Milliseconds()

	return plan
}

// decomposeTask breaks a task description into actionable steps.
// This is a heuristic decomposition; in production it would use an LLM.
func (tp *TaskPlanner) decomposeTask(input string) []event.TaskPlanStep {
	inputLower := strings.ToLower(input)
	var steps []event.TaskPlanStep
	stepID := 0

	addStep := func(desc, tools, outcome, risk string) {
		stepID++
		if stepID > tp.maxSteps {
			return
		}
		steps = append(steps, event.TaskPlanStep{
			ID:              stepID,
			Description:     desc,
			ToolsNeeded:     tools,
			ExpectedOutcome: outcome,
			RiskLevel:       risk,
		})
	}

	// CTF-specific task decomposition
	isRecon := strings.Contains(inputLower, "recon") ||
		strings.Contains(inputLower, "scan") ||
		strings.Contains(inputLower, "enum") ||
		strings.Contains(inputLower, "port") ||
		strings.Contains(inputLower, "fingerprint") ||
		strings.Contains(inputLower, "information gather")

	isExploit := strings.Contains(inputLower, "exploit") ||
		strings.Contains(inputLower, "attack") ||
		strings.Contains(inputLower, "rce") ||
		strings.Contains(inputLower, "shell") ||
		strings.Contains(inputLower, "reverse") ||
		strings.Contains(inputLower, "pwn") ||
		strings.Contains(inputLower, "exec")

	isWeb := strings.Contains(inputLower, "web") ||
		strings.Contains(inputLower, "http") ||
		strings.Contains(inputLower, "url") ||
		strings.Contains(inputLower, "xss") ||
		strings.Contains(inputLower, "sqli") ||
		strings.Contains(inputLower, "csrf")

	isCrypto := strings.Contains(inputLower, "crypto") ||
		strings.Contains(inputLower, "encrypt") ||
		strings.Contains(inputLower, "rsa") ||
		strings.Contains(inputLower, "aes") ||
		strings.Contains(inputLower, "hash") ||
		strings.Contains(inputLower, "cipher")

	isReverse := strings.Contains(inputLower, "reverse") ||
		strings.Contains(inputLower, "disassem") ||
		strings.Contains(inputLower, "decompile") ||
		strings.Contains(inputLower, "binary") ||
		strings.Contains(inputLower, "analysis") ||
		strings.Contains(inputLower, "strings")

	isForensic := strings.Contains(inputLower, "forensic") ||
		strings.Contains(inputLower, "memory") ||
		strings.Contains(inputLower, "volatility") ||
		strings.Contains(inputLower, "disk") ||
		strings.Contains(inputLower, "pcap")

	// Generate domain-specific step sequences
	switch {
	case isRecon:
		addStep("Information gathering and target profiling",
			"bash, web_fetch", "Comprehensive list of open ports and services", "low")
		addStep("Service version detection and fingerprinting",
			"bash, web_fetch", "Identified service versions and potential vulnerabilities", "low")
		addStep("Vulnerability research for identified services",
			"web_fetch, search", "Mapped CVEs and known exploits for target services", "medium")
		if isWeb {
			addStep("Web application enumeration",
				"bash, web_fetch", "Discovered web paths, technologies, and entry points", "medium")
		}
		addStep("Cross-reference and priority mapping",
			"bash", "Prioritized list of attack vectors based on findings", "low")

	case isExploit:
		addStep("Select and prepare exploit for target vulnerability",
			"bash, web_fetch", "Exploit payload ready for delivery", "high")
		addStep("Deliver exploit and establish foothold",
			"bash", "Initial access or shell obtained on target", "critical")
		addStep("Post-exploitation enumeration",
			"bash", "Collected system info, credentials, and sensitive data", "high")
		addStep("Privilege escalation analysis",
			"bash", "Escalated privileges to target level", "high")
		addStep("Data exfiltration and persistence (if required)",
			"bash", "Flag/data captured and secure channel established", "high")

	case isCrypto:
		addStep("Identify cipher/hash type and parameters",
			"bash, python", "Determined encryption algorithm and key parameters", "low")
		addStep("Analyze cryptographic weaknesses",
			"bash, python", "Identified implementation flaws or mathematical weaknesses", "medium")
		addStep("Develop or apply decryption attack",
			"bash, python", "Decrypted ciphertext or reversed hash", "medium")
		addStep("Validate recovered plaintext",
			"bash", "Verified decrypted data matches expected format", "low")

	case isReverse:
		addStep("Extract strings and metadata from binary",
			"bash", "Collected hints, embedded strings, and metadata", "low")
		addStep("Disassemble and analyze core logic",
			"bash", "Understood program control flow and key routines", "medium")
		addStep("Identify vulnerability or hidden functionality",
			"bash", "Found the exploit path or hidden flag logic", "medium")
		addStep("Develop and test solution",
			"bash, python", "Working exploit or flag extraction script", "high")

	case isForensic:
		addStep("Identify evidence sources and extract artifacts",
			"bash", "Extracted key artifacts from forensic data", "low")
		addStep("Analyze timeline and correlate events",
			"bash", "Built timeline of relevant events", "medium")
		addStep("Recover deleted/hidden data",
			"bash", "Recovered key evidence from unallocated space", "medium")
		addStep("Summarize findings",
			"bash", "Complete forensic analysis report", "low")

	default:
		// Generic task decomposition
		addStep("Understand the task and identify required information",
			"bash, web_fetch", "Clear understanding of requirements and approach", "low")
		addStep("Gather necessary context and dependencies",
			"bash, web_fetch", "Collected all prerequisite information", "low")
		addStep("Implement the core solution",
			"bash, python, edit_file", "Working implementation of the solution", "medium")
		addStep("Verify and validate results",
			"bash", "Confirmed correct output and captured evidence", "low")
	}

	return steps
}

// classifyTaskStrategy determines the overall strategy description.
func classifyTaskStrategy(input string) string {
	inputLower := strings.ToLower(input)

	switch {
	case strings.Contains(inputLower, "recon") || strings.Contains(inputLower, "scan") || strings.Contains(inputLower, "enum"):
		return "Reconnaissance and enumeration — systematically map the attack surface"
	case strings.Contains(inputLower, "exploit") || strings.Contains(inputLower, "rce") || strings.Contains(inputLower, "shell") || strings.Contains(inputLower, "pwn"):
		return "Exploitation — identify and leverage vulnerabilities to gain access"
	case strings.Contains(inputLower, "crypto") || strings.Contains(inputLower, "encrypt") || strings.Contains(inputLower, "decrypt"):
		return "Cryptanalysis — analyze and break cryptographic implementations"
	case strings.Contains(inputLower, "reverse") || strings.Contains(inputLower, "binary"):
		return "Reverse engineering — analyze binary to understand its logic and find vulnerabilities"
	case strings.Contains(inputLower, "forensic") || strings.Contains(inputLower, "memory"):
		return "Digital forensics — extract and analyze evidence from forensic artifacts"
	case strings.Contains(inputLower, "web") || strings.Contains(inputLower, "http"):
		return "Web application testing — identify and exploit web vulnerabilities"
	default:
		return "General purpose — systematically work through the task from information gathering to solution delivery"
	}
}

// assessTaskRisk evaluates the overall risk level of the task.
func assessTaskRisk(input string) string {
	inputLower := strings.ToLower(input)

	highRiskKeywords := []string{"exploit", "rce", "shell", "reverse", "exec", "delete", "modify", "inject"}
	mediumRiskKeywords := []string{"scan", "enumerate", "brute", "crack", "decrypt", "dump"}

	for _, kw := range highRiskKeywords {
		if strings.Contains(inputLower, kw) {
			return "high — involves active exploitation or system modification"
		}
	}
	for _, kw := range mediumRiskKeywords {
		if strings.Contains(inputLower, kw) {
			return "medium — involves active probing or analysis with potential side effects"
		}
	}
	return "low — primarily information gathering and passive analysis"
}

// FormatPlan formats the plan as a human-readable string for terminal output.
func FormatPlan(plan *event.TaskPlanResult) string {
	if plan == nil || len(plan.Steps) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("📋 Task Plan\n")
	b.WriteString(fmt.Sprintf("Strategy: %s\n", plan.Strategy))
	if plan.RiskAssessment != "" {
		b.WriteString(fmt.Sprintf("Risk: %s\n", plan.RiskAssessment))
	}
	b.WriteString(fmt.Sprintf("Estimated budget: ~%d tool calls\n", plan.TotalEstimated))
	b.WriteString("\nSteps:\n")
	for _, step := range plan.Steps {
		b.WriteString(fmt.Sprintf("  %d. %s\n", step.ID, step.Description))
		if step.ToolsNeeded != "" {
			b.WriteString(fmt.Sprintf("     Tools: %s\n", step.ToolsNeeded))
		}
		if step.ExpectedOutcome != "" {
			b.WriteString(fmt.Sprintf("     → %s\n", step.ExpectedOutcome))
		}
	}
	return b.String()
}
