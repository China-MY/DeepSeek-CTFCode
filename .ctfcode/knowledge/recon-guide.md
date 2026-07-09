---
name: recon-guide
description: 信息收集与漏洞扫描全流程指南 — 资产测绘→端口扫描→指纹识别→漏洞扫描→弱口令爆破
tags: [recon, scanning, information-gathering]
agents: [planner, recon]
---

# 信息收集与漏洞扫描全流程指南

## 阶段 1：信息收集

| 目的 | 工具/方法 | 产出 |
|------|----------|------|
| 资产测绘 | FOFA / TscanPlus cyber_search | 域名、IP、技术栈 |
| 子域名 | TscanPlus subdomain_scan / ez dnsscan | 子域名清单 |
| 端口扫描 | TscanPlus ip_scan / ez servicescan / nmap | 开放端口、服务版本 |
| JS泄露 | TscanPlus js_scan / gau / katana | 隐藏接口、密钥、硬编码凭证 |
| CVE匹配 | poc-lab / exploitarium grep | 版本→CVE映射 |

### 端口扫描规范
- 每台主机至少扫 Top 1000 端口
- 关键目标扫 1-65535 全端口
- 非标端口必须深入测试（HTTP、数据库、自定义协议）
- 使用 nmap -sV 做服务版本识别

### 子域名收集
- TscanPlus subdomain_scan
- subfinder 被动枚举
- 从 JS 文件中提取子域名

## 阶段 2：漏洞扫描与指纹识别

| 操作 | 工具 |
|------|------|
| URL指纹 | TscanPlus url_scan finger=all |
| 目录枚举 | TscanPlus dir_scan / ffuf / dirsearch |
| Web漏洞 | ez webscan / TscanPlus poc_scan / nuclei |
| 参数发现 | arjun / x8 / paramspider |
| 弱口令 | hydra HTTP表单模式 / TscanPlus pwd_crack |
| 加密API逆向 | 前端JS提取key/iv → Python复现加密 |

### 加固登录页面弱口令爆破
每个登录表单的Web系统必须进行弱口令爆破：
```
hydra -L users.txt -P /usr/share/wordlists/rockyou.txt <target> http-post-form "/login:user=^USER^&pass=^PASS^:F=incorrect"
```

### 交叉验证规则
- 每个漏洞至少两种工具/方法独立确认
- 无法复现的漏洞标注 D 级（误报），不可删除
- 指纹信息必须直接映射到 CVE Exploit 脚本

## SRC 高频方向
OA系统(致远/泛微/用友) · Shiro · Fastjson · Spring Actuator · API越权 · JS密钥泄露 · ArcGIS Server · 加密API(AES/SM4)绕过 · 弱口令爆破

## CVSS 3.1 评分速查
| 等级 | 分数 | 判定标准 |
|------|------|---------|
| 严重 | 9.0-10.0 | 远程无需认证RCE/数据全量泄露/完全控制 |
| 高危 | 7.0-8.9 | 越权敏感数据/SQL注入/认证绕过/文件上传RCE |
| 中危 | 4.0-6.9 | XSS/目录遍历/信息泄露 |
| 低危 | 0.1-3.9 | 文本信息泄露 |
| 信息 | 0.0 | 技术栈指纹/版本号 |

简化为：能弹shell/RCE/拖库 → Critical/High
