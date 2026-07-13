# Changelog

All notable changes to the Go line (Reasonix 1.0+) are recorded here. The legacy
`0.x` TypeScript history lives on the [`v1`](https://github.com/esengine/DeepSeek-Reasonix/tree/v1)
branch.

## [1.10.0] — 2026-07-13

### Added

- **AI渗透系统重塑**：全面品牌升级为「AI渗透系统 — Pentest Jarvis」，基于《AI渗透系统白皮书》和产品介绍文档改造。
- **新增 pentest 后端包**：`internal/pentest/` 包含 TaskNode（任务树节点）、VulnerabilityCard（漏洞证据卡片）、PentestReport（渗透报告）、AISkill（可复用攻击技能）、CTFChallenge（CTF挑战）等数据模型及完整 REST API（15个端点）。
- **专业渗透工作台 Web UI**：完全重设计 index.html（1643行），包含：
  - 仪表盘：渗透态势感知，实时显示目标数、漏洞数、任务进度、攻击路径
  - 工作台：AI实时对话，流式展示推理过程与工具调用日志
  - 任务树：可视化结构化攻击路径，节点状态跟踪
  - 漏洞卡片：漏洞证据链展示（严重等级、Payload、Shell回显）
  - 报告生成：一键导出渗透测试报告
  - CTF挑战：全题型辅助面板
- **双Agent协同架构**：提示词全面改写为「任务拆解Agent」+「动态执行Agent」+「报告固化Agent」架构。

### Changed

- 品牌从 "ctfcode / Reasonix" 升级为 "AI渗透系统 — Pentest Jarvis"
- 配置系统 (`ctfcode.toml`) 更新品牌名称和 Agent 角色描述
- Agent 提示词 (`planner.md`, `recon.md`, `exploit.md`, `report.md`) 全面重写
- README.md 更新为 AI渗透系统 架构说明
- 环境变量从 `REASONIX_CTF_PHASE` 改为 `PENTEST_PHASE`

### Fixed

- Web UI 修复中危漏洞统计显示语法错误

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
