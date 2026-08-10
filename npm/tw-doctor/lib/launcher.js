"use strict";

const childProcess = require("node:child_process");

const binaryPackages = Object.freeze({
  "darwin-arm64": "@tw-doctor/darwin-arm64",
  "darwin-x64": "@tw-doctor/darwin-x64",
  "linux-arm64": "@tw-doctor/linux-arm64",
  "linux-x64": "@tw-doctor/linux-x64",
  "win32-x64": "@tw-doctor/win32-x64",
});

function binaryPackage(platform, architecture) {
  return binaryPackages[`${platform}-${architecture}`] || null;
}

function main(arguments_, runtime = {}) {
  const platform = runtime.platform || process.platform;
  const architecture = runtime.architecture || process.arch;
  const resolve = runtime.resolve || require.resolve;
  const spawnSync = runtime.spawnSync || childProcess.spawnSync;
  const writeError = runtime.writeError || ((message) => console.error(message));
  const packageName = binaryPackage(platform, architecture);

  if (!packageName) {
    writeError(
      `tw-doctor: unsupported platform ${platform}/${architecture}; ` +
        `supported targets are ${Object.keys(binaryPackages).join(", ")}`,
    );
    return 2;
  }

  const executable = platform === "win32" ? "tw-doctor.exe" : "tw-doctor";
  let binary;
  try {
    binary = resolve(`${packageName}/bin/${executable}`);
  } catch (error) {
    writeError(
      `tw-doctor: the optional binary package ${packageName} is missing; ` +
        `reinstall tw-doctor for ${platform}/${architecture}`,
    );
    return 2;
  }

  const result = spawnSync(binary, arguments_, { stdio: "inherit" });
  if (result.error) {
    writeError(`tw-doctor: could not start ${binary}: ${result.error.message}`);
    return 2;
  }
  if (result.signal) {
    process.kill(process.pid, result.signal);
    return 2;
  }
  return result.status === null ? 2 : result.status;
}

module.exports = { binaryPackage, main };
