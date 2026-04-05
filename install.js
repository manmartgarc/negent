#!/usr/bin/env node

const https = require("https");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

const REPO = "manmart/negent";

function getPlatform() {
  switch (process.platform) {
    case "darwin":
      return "darwin";
    case "linux":
      return "linux";
    default:
      throw new Error(`Unsupported platform: ${process.platform}`);
  }
}

function getArch() {
  switch (process.arch) {
    case "x64":
      return "amd64";
    case "arm64":
      return "arm64";
    default:
      throw new Error(`Unsupported architecture: ${process.arch}`);
  }
}

function httpGet(url, options = {}) {
  return new Promise((resolve, reject) => {
    const opts = {
      headers: { "User-Agent": "negent-installer" },
      ...options,
    };

    https
      .get(url, opts, (response) => {
        if (response.statusCode === 301 || response.statusCode === 302) {
          httpGet(response.headers.location, options).then(resolve, reject);
          return;
        }

        if (response.statusCode !== 200) {
          reject(
            new Error(`HTTP ${response.statusCode} fetching ${url}`)
          );
          return;
        }

        const chunks = [];
        response.on("data", (chunk) => chunks.push(chunk));
        response.on("end", () => resolve(Buffer.concat(chunks)));
      })
      .on("error", reject);
  });
}

async function getLatestVersion() {
  const data = await httpGet(
    `https://api.github.com/repos/${REPO}/releases/latest`
  );
  const release = JSON.parse(data.toString());
  if (!release.tag_name) {
    throw new Error("No release found");
  }
  return release.tag_name;
}

async function install() {
  const binDir = path.join(__dirname, "bin");
  const binaryPath = path.join(binDir, "negent");
  const tarPath = path.join(binDir, "negent.tar.gz");

  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  try {
    const platform = getPlatform();
    const arch = getArch();

    console.log("Fetching latest version...");
    const version = await getLatestVersion();
    const assetName = `negent-${version}-${platform}-${arch}.tar.gz`;
    const url = `https://github.com/${REPO}/releases/download/${version}/${assetName}`;

    console.log(`Downloading negent ${version} (${platform}-${arch})...`);
    const data = await httpGet(url);
    fs.writeFileSync(tarPath, data);

    // Extract the negent binary from the tarball
    execSync(`tar -xzf "${tarPath}" -C "${binDir}" negent`, {
      stdio: "pipe",
    });
    fs.unlinkSync(tarPath);
    fs.chmodSync(binaryPath, 0o755);

    console.log(`negent ${version} installed successfully.`);
  } catch (err) {
    console.error(`Failed to install negent: ${err.message}`);
    console.error(
      `\nYou can manually download from:\n  https://github.com/${REPO}/releases/latest`
    );
    process.exit(1);
  }
}

install();
