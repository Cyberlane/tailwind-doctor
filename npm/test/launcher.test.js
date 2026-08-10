"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");

const { binaryPackage, main } = require("../tw-doctor/lib/launcher");

test("maps every supported Node target to its binary package", () => {
  assert.equal(binaryPackage("darwin", "arm64"), "@tw-doctor/darwin-arm64");
  assert.equal(binaryPackage("darwin", "x64"), "@tw-doctor/darwin-x64");
  assert.equal(binaryPackage("linux", "arm64"), "@tw-doctor/linux-arm64");
  assert.equal(binaryPackage("linux", "x64"), "@tw-doctor/linux-x64");
  assert.equal(binaryPackage("win32", "x64"), "@tw-doctor/win32-x64");
  assert.equal(binaryPackage("freebsd", "x64"), null);
});

test("forwards arguments and the child exit status", () => {
  let resolved;
  let spawned;
  const status = main(["--json", "."], {
    platform: "linux",
    architecture: "arm64",
    resolve(specifier) {
      resolved = specifier;
      return "/package/bin/tw-doctor";
    },
    spawnSync(binary, arguments_, options) {
      spawned = { binary, arguments_, options };
      return { status: 1, signal: null };
    },
  });

  assert.equal(resolved, "@tw-doctor/linux-arm64/bin/tw-doctor");
  assert.deepEqual(spawned, {
    binary: "/package/bin/tw-doctor",
    arguments_: ["--json", "."],
    options: { stdio: "inherit" },
  });
  assert.equal(status, 1);
});

test("uses the Windows executable name", () => {
  let resolved;
  const status = main([], {
    platform: "win32",
    architecture: "x64",
    resolve(specifier) {
      resolved = specifier;
      return "C:\\package\\bin\\tw-doctor.exe";
    },
    spawnSync() {
      return { status: 0, signal: null };
    },
  });
  assert.equal(resolved, "@tw-doctor/win32-x64/bin/tw-doctor.exe");
  assert.equal(status, 0);
});

test("fails closed for unsupported or missing packages", () => {
  const errors = [];
  const unsupported = main([], {
    platform: "freebsd",
    architecture: "x64",
    writeError: (message) => errors.push(message),
  });
  const missing = main([], {
    platform: "darwin",
    architecture: "arm64",
    resolve() {
      throw new Error("not found");
    },
    writeError: (message) => errors.push(message),
  });

  assert.equal(unsupported, 2);
  assert.equal(missing, 2);
  assert.match(errors[0], /unsupported platform freebsd\/x64/);
  assert.match(errors[1], /@tw-doctor\/darwin-arm64 is missing/);
});
