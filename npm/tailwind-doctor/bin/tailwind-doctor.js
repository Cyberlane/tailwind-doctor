#!/usr/bin/env node

const { main } = require("tw-doctor/lib/launcher");

process.exitCode = main(process.argv.slice(2));
