#!/usr/bin/env node
// ctfcode postinstall — 从 GitHub Releases 下载预编译二进制
const https = require("node:https");
const fs = require("node:fs");
const path = require("node:path");
const zlib = require("node:zlib");

const REPO = "China-MY/DeepSeek-CTFCode";
const VERSION = "v1.0.0";
const BIN_DIR = path.join(__dirname, "..", "bin");
const EXE_NAME = process.platform === "win32" ? "ctfcode.exe" : "ctfcode";
const BIN_PATH = path.join(BIN_DIR, EXE_NAME);

// 已有二进制则跳过
if (fs.existsSync(BIN_PATH)) {
  process.exit(0);
}

// 平台映射
const platformMap = {
  "linux-x64": "linux-amd64",
  "linux-arm64": "linux-arm64",
  "darwin-x64": "darwin-x64",
  "darwin-arm64": "darwin-arm64",
  "win32-x64": "windows-amd64",
  "win32-arm64": "windows-arm64",
};

const mapped = platformMap[`${process.platform}-${process.arch}`];
if (!mapped) {
  console.error(`ctfcode: 不支持的平台 ${process.platform}-${process.arch}`);
  process.exit(1);
}

const downloadUrl = `https://github.com/${REPO}/releases/download/${VERSION}/ctfcode-${mapped}`;

console.log(`ctfcode: 下载预编译二进制 (${mapped})...`);

fs.mkdirSync(BIN_DIR, { recursive: true });

// 从 GitHub Release 下载二进制（原始文件，非 tar.gz）
https.get(downloadUrl, { timeout: 30000 }, (res) => {
  // GitHub Release 原始文件下载可能有重定向
  if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
    https.get(res.headers.location, { timeout: 30000 }, (res2) => {
      if (res2.statusCode !== 200) {
        console.error(`ctfcode: 下载失败 (HTTP ${res2.statusCode})`);
        process.exit(1);
      }
      const fileStream = fs.createWriteStream(BIN_PATH);
      res2.pipe(fileStream);
      fileStream.on("finish", () => {
        fileStream.close();
        fs.chmodSync(BIN_PATH, 0o755);
        console.log(`ctfcode: 安装完成！二进制路径: ${BIN_PATH}`);
      });
    });
    return;
  }
  if (res.statusCode !== 200) {
    // 尝试备用：从 Release tarball 中提取
    const fallbackUrl = `https://github.com/${REPO}/archive/${VERSION}.tar.gz`;
    console.log(`ctfcode: 尝试从源码构建... 请运行: npm install -g ${REPO}`);
    console.log(`ctfcode: 或手动下载: ${downloadUrl}`);
    process.exit(1);
  }
  const fileStream = fs.createWriteStream(BIN_PATH);
  res.pipe(fileStream);
  fileStream.on("finish", () => {
    fileStream.close();
    fs.chmodSync(BIN_PATH, 0o755);
    console.log(`ctfcode: 安装完成！二进制路径: ${BIN_PATH}`);
  });
}).on("error", (err) => {
  console.error(`ctfcode: 下载失败: ${err.message}`);
  process.exit(1);
});
