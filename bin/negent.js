#!/usr/bin/env node

const { spawn } = require("child_process");
const path = require("path");
const fs = require("fs");

function getBinaryPath() {
  const platform = process.platform;
  const binaryName = platform === "win32" ? "negent.exe" : "negent";
  const binaryPath = path.join(__dirname, binaryName);

  if (!fs.existsSync(binaryPath)) {
    console.error("Error: negent binary not found.");
    console.error("Try reinstalling: npm install -g negent");
    process.exit(1);
  }

  return binaryPath;
}

const binary = getBinaryPath();
const args = process.argv.slice(2);

const child = spawn(binary, args, {
  stdio: "inherit",
  env: process.env,
});

child.on("error", (err) => {
  console.error(`Failed to start negent: ${err.message}`);
  process.exit(1);
});

child.on("close", (code) => {
  process.exit(code || 0);
});
