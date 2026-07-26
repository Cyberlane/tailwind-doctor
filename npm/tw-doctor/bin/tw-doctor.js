#!/usr/bin/env node

// Version 0.0.0 exists only to hold the package name. It intentionally ships no
// binary and no download step, so running it explains itself instead of failing
// with a confusing "command not found".
//
// The real launcher lands with the first tagged release: it resolves a prebuilt
// Go binary from a platform-specific optional dependency and forwards argv to it.
// Exit code 2 matches the CLI's operational-error code.

console.error(
  [
    "tw-doctor has not been released yet.",
    "This 0.0.0 publish reserves the package name; it contains no binary.",
    "",
    "To try the current development version, build it from source:",
    "  go run ./cmd/tw-doctor .",
    "",
    "https://github.com/Cyberlane/tailwind-doctor",
  ].join("\n")
);

process.exit(2);
