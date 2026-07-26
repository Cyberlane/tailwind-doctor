#!/usr/bin/env node

const { spawnSync } = require("node:child_process");

const binary = process.platform === "win32" ? "tw-doctor.exe" : "tw-doctor";
const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error("tw-doctor binary was not found. Install a release binary or use the official npm package once published.");
  process.exit(1);
}

process.exit(result.status ?? 1);
