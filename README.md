# DeepSeek-CTFCode

<p align="center">
  <strong>🚀 CTF渗透测试多Agent协作系统 — 基于 DeepSeek 的智能渗透测试平台</strong>
</p>

<p align="center">
  <em>作者：Mr.明裕</em>
</p>

## 概述

DeepSeek-CTFCode 是一个基于 DeepSeek 大语言模型驱动的**多Agent协作渗透测试系统**。它将渗透测试全流程拆分为四个专业化 Agent 角色，通过 handoff 机制协同工作，实现从信息收集到漏洞利用到报告生成的全自动化。

### 核心原则
- **Python Exploit 才能证明漏洞可利用** — 自动化工具只能发现漏洞
- **中危/低危漏洞禁止直接汇报** — 必须尝试危害提升后再定级
- **严格报告格式** — Tab分隔的固定字段格式

## 架构

```
用户输入 → Planner(规划师)  — deepseek-v4-pro
            ├─ handoff → Recon(侦察兵)    — 信息收集/端口扫描/指纹识别  — deepseek-v4-flash
            ├─ handoff → Exploit(利用手)   — Python Exploit开发/实际利用 — deepseek-v4-flash
            └─ handoff → Report(报告员)    — 证据整理/漏洞定级/报告输出 — deepseek-v4-pro
```

| Agent | 模型 | 职责 | 工具权限 |
|-------|------|------|---------|
| **Planner** | deepseek-v4-pro | 任务拆解、Agent调度、全局控制 | 只读 + handoff |
| **Recon** | deepseek-v4-flash | 资产测绘、端口扫描、JS分析、CVE匹配 | bash + 扫描工具 |
| **Exploit** | deepseek-v4-flash | Python Exploit开发、危害提升、证据提取 | bash + 文件读写 |
| **Report** | deepseek-v4-pro | 危害复核、漏洞定级、固定格式报告 | 只读 + 文件写入 |

## 快速开始

### 1. 直接安装（推荐）

```bash
# 通过 npm 从 GitHub 直接安装（自动下载预编译二进制）
npm install -g github:China-MY/DeepSeek-CTFCode

# 验证安装
ctfcode --version
```

### 2. 设置 API 密钥

```bash
export DEEPSEEK_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### 3. 从源码编译（可选）

```bash
cd DeepSeek-CTFCode
make build
```

### 4. 运行

```bash
# 交互式会话（推荐）
./bin/ctfcode

# 单次任务执行
./bin/ctfcode run "对 target.com 进行渗透测试"

# 指定模型
./bin/ctfcode --model deepseek-v4-flash
```

### 5. 用户配置

首次运行会自动创建 `~/.ctfcode/` 目录。配置文件加载顺序：

```
./ctfcode.toml > ~/.ctfcode/config.toml > 内置默认值
```

密钥通过环境变量注入（如 `DEEPSEEK_API_KEY`），永不写入配置文件。

## 功能特性

### 🎯 多Agent协作
- 四角色专业化分工，handoff 机制自动流转
- 每个 Agent 可绑定不同模型（pro 做规划/报告，chat 做执行/扫描）

### 🔍 全面信息收集
- 资产测绘（FOFA / TscanPlus）
- 全端口扫描 + 服务版本精确识别
- 技术栈指纹精确到小版本号 → CVE匹配
- JS 深层分析（API Key、secret、端点提取）
- 云/CDN 检测与真实 IP 探测

### 💥 Python Exploit 实际利用
- 15+ 种技术栈→Exploit 映射（Struts2/Shiro/Fastjson/ThinkPHP/Spring Boot/OA等）
- 三阶段利用法：触发验证 → 深度利用 → 危害提升
- WAF 绕过策略（编码/速率/签名/云WAF）

### ⬆️ 漏洞危害提升
- SQL注入→RCE 路径（xp_cmdshell/INTO OUTFILE/LOAD_FILE）
- 文件读取→RCE 路径（日志投毒/密钥复用/反序列化）
- XSS→权限提升（凭据窃取/CSRF配合）
- 漏洞链组合（2×中危→高危）
- 所有提升尝试失败后才按原始等级报告

### 📋 严格报告格式
```
项目    {项目名称}
漏洞详情    {漏洞名称}
漏洞风险等级    严重/高危/中危/低危/信息
漏洞地址    {URL}
漏洞描述与原理    {描述}
验证步骤    见附件 Python 验证脚本
利用方式与影响    {攻击者视角描述}
临时缓解措施    {临时方案}
永久修复建议    {永久方案}
```

### 🖥️ 实时状态栏
```
🔍 RECON  ● 🔍 ───○ 💥───○ 📋  |  deepseek-v4-flash  |  bal:¥110.00
💥 EXPLOIT  ○ 🔍 ───● 💥───○ 📋  |  deepseek-v4-flash  |  bal:¥110.00
📋 REPORT  ○ 🔍 ───○ 💥───● 📋  |  deepseek-v4-pro  |  bal:¥110.00
```

- 实时显示渗透测试节点流程
- 余额自动查询（通过 DeepSeek API）
- 阶段状态由 Agent 自动更新

### 🧠 自学习
- Agent 自动记录经验和教训
- 跨会话回顾有用模式
- 避免重复犯错

## 配置参考

### 完整配置示例

```toml
# ctfcode.toml — CTF渗透测试多Agent协作系统

default_model = "deepseek"
language = "zh"

[ui]
theme = "dark"
theme_style = "aurora"

[agent]
system_prompt = """
你是 ctfcode — CTF渗透测试和安全研究AI Agent。
"""
temperature = 0.0
auto_plan = "on"
output_style = "ctf-pentest"

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
balance_url = "https://api.deepseek.com/user/balance"

[[providers]]
name = "deepseek-pro"
kind = "openai"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-pro"
api_key_env = "DEEPSEEK_API_KEY"
balance_url = "https://api.deepseek.com/user/balance"

# 多Agent配置 - 参见 .ctfcode/prompts/ 下的提示词文件
[[agents]]
id = "planner"
name = "规划师"
model = "deepseek-v4-pro"
# ...
```

## 产出规范

所有发现写入 `/root/ATKING/`：

```
/root/ATKING/
├── 报告md/{项目名}-渗透测试报告.md    ← 严格格式报告
├── exploit_{漏洞类型}.py               ← Python 利用脚本
├── etc_passwd                          ← 下载的系统文件
└── env_file                            ← 下载的配置信息
```

## 命令参考

| 命令 | 说明 |
|------|------|
| `ctfcode` | 启动交互式会话 |
| `ctfcode run "任务"` | 单次任务执行 |
| `ctfcode --version` | 显示版本信息 |
| `ctfcode setup` | 配置向导 |
| `/init` | 生成项目记忆文件 |
| `/clear` | 清除会话历史 |
| `/mcp` | 管理 MCP 插件 |

## 项目结构

```
DeepSeek-CTFCode/
├── bin/ctfcode                    # 编译产物
├── cmd/ctfcode/                   # CLI 入口
├── internal/                      # 核心代码
│   ├── agent/                     # Agent 机制 + 多Agent编排
│   ├── tool/                      # 工具系统
│   ├── provider/                  # AI 提供商适配
│   ├── config/                    # 配置系统
│   ├── skill/                     # 技能系统
│   ├── knowledge/                 # 知识库引擎
│   └── billing/                   # 计费/余额
├── .ctfcode/                      # 项目级配置
│   ├── prompts/                   # Agent 系统提示词
│   ├── knowledge/                 # 知识库条目
│   └── statusline.sh              # 状态栏脚本
├── ctfcode.toml                   # 主配置文件
├── Makefile                       # 构建系统
├── package.json                   # npm 入口
└── go.mod                         # Go 模块
```

## 致谢

感谢 [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix.git) 原项目的出色工作，为本项目提供了坚实的基础与灵感。

## 作者

- **Mr.明裕** — 项目作者与维护者

## 许可证

本项目基于 MIT 许可证开源。
