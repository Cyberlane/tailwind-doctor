#!/usr/bin/env node

// `npx tailwind-doctor` resolves a package named tailwind-doctor, so a bin alias
// declared inside the tw-doctor package is not enough on its own. This package
// exists to make both invocation names work, published in lockstep with
// tw-doctor at the same version.
//
// At 0.0.0 it holds the name and nothing more. It deliberately does not depend
// on tw-doctor yet, so the two placeholders can be published in any order and
// neither pulls a second non-functional package into an install.

console.error(
  [
    "tailwind-doctor has not been released yet.",
    "This 0.0.0 publish reserves the package name; it contains no binary.",
    "",
    "To try the current development version, build it from source:",
    "  go run ./cmd/tw-doctor .",
    "",
    "https://github.com/Cyberlane/tailwind-doctor",
  ].join("\n")
);

process.exit(2);
