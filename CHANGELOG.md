# Changelog

All notable changes to the Go line (Reasonix 1.0+) are recorded here. The legacy
`0.x` TypeScript history lives on the [`v1`](https://github.com/esengine/DeepSeek-Reasonix/tree/v1)
branch.

## Unreleased

### Added

- **执行监督系统 (Adviser)**：参考 PentAGI Execution Monitoring，实时监控 Agent 工具调用模式。自动检测循环（相同工具连续调用N次）、错误重复、总调用阈值，通过 Steer 机制注入纠正指导。
- **反射器 (Reflector)**：参考 PentAGI Reflector Integration，当 Agent 遇到重复失败时自动介入。支持 tool_error/empty_turn/plan_stuck/loop 四种失败模式分类，匹配 timeout/permission/connection 等错误模式生成根因分析和纠正建议。
- **智能任务规划 (TaskPlanner)**：参考 PentAGI Intelligent Task Planning，在复杂任务执行前自动生成结构化计划。支持 Recon/Exploit/Web/Crypto/Reverse/Forensic 六类 CTF 场景的领域特定任务分解，含风险评估和工具建议。
- **事件系统扩展**：新增 `AdviserAssessment`、`ReflectorAssessment`、`TaskPlan` 三种事件类型和完整的 payload 结构体，TUI 支持可视化展示。
- **配置系统**：`AgentConfig` 新增 `ExecutionMonitorConfig`、`ReflectorConfig`、`TaskPlannerConfig` 三组 TOML 配置项。

### Changed

- Agent 主循环集成 Adviser/Reflector/TaskPlanner 三组件，支持可配置启用。
- Boot 装配流程新增 8 个配置读取助手函数。
- ctfcode.toml 默认启用上述监控功能。
- Agent runtime defaults now leave both executor and dedicated planner tool-call
  rounds unlimited (`max_steps = 0`, `planner_max_steps = 0`). Step limits now
  come from the user/global config only; project `reasonix.toml` does not
  override them.

## [1.0.0] — 2026-06-03

First stable release — a **ground-up rewrite in Go**. Not an upgrade of the `0.x`
TypeScript line; a new codebase that becomes the default (`main-v2`).

### Highlights

- **Go kernel**: a single static binary (CGO-free), cross-compiled for
  darwin/linux/windows on amd64 + arm64. Distributed via npm (the package wraps
  the native binary), Homebrew (`esengine/reasonix` tap), and release archives;
  no Node runtime needed to run it.
- **Agent core**: the loop, built-in tools (read/write/edit/multi_edit/glob/grep/
  ls/bash/web_fetch/todo_write), permission gate, sandboxed bash, and the
  DeepSeek prefix-cache–oriented design.
- **Subagents**: `task` plus explore/research/review/security_review skill agents.
- **Skills & hooks**: Claude-Code-style skills (`internal/skill`) and hooks
  (`internal/hook`), symlink-aware and slash-integrated.
- **MCP client**: connect external servers over stdio / Streamable HTTP; reads
  `[[plugins]]` and a Claude-Code `.mcp.json`.
- **Code intelligence via CodeGraph**: a tree-sitter symbol/call graph
  (`codegraph_*` tools) replaces embedding semantic search — no embedding service
  or API cost. Fetched into a local cache on first use (or `reasonix codegraph
  install`) and indexed in the background, so installs and startup stay fast.
- **Plan mode** with evidence-backed step sign-off (`complete_step`).
- **Memory**: `REASONIX.md` hierarchy + auto-memory, folded into the cache-stable
  prefix.
- **ACP** (`reasonix acp`) and an HTTP/SSE server frontend; desktop app (Wails).

### Fixed

- **File encoding support restored** — GBK/GB18030 (and other non-UTF-8) files
  can now be read, edited, and grepped correctly. The v2 rewrite had dropped
  v1's encoding detection; files in CJK Windows charsets were silently misread
  or rejected as binary. The read/edit/write round-trip now preserves the
  original file encoding. (#2637)

### Notes

- Versions: the legacy TypeScript line stays in `0.x`; the Go line starts at
  `1.0.0`. See [docs/MIGRATING.md](docs/MIGRATING.md).
- Release archives ship a bare binary; CodeGraph is fetched on first use. Windows
  support for the fetched runtime is unverified — install `codegraph` on PATH if
  the auto-fetch doesn't resolve there.

[1.0.0]: https://github.com/esengine/DeepSeek-Reasonix/releases/tag/v1.0.0
