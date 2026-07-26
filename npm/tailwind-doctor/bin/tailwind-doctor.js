#!/usr/bin/env node

// `npx tailwind-doctor` resolves a package named tailwind-doctor, so a bin alias
// declared inside the tw-doctor package is not enough on its own. This package
// exists only to forward to the real launcher, and is published in lockstep with
// tw-doctor at the same version.

require("tw-doctor/bin/tw-doctor.js");
