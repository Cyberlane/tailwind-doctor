"use strict";

const assert = require("node:assert/strict");
const Module = require("node:module");
const test = require("node:test");

const originalLoad = Module._load;
Module._load = function load(request, parent, isMain) {
  if (request === "vscode") return {};
  return originalLoad.call(this, request, parent, isMain);
};
const { _testing } = require("./extension.js");
Module._load = originalLoad;

function document(languageId, path) {
  return { languageId, uri: { path } };
}

test("handles source and project-context saves", () => {
  assert.equal(_testing.shouldHandleSave(document("typescriptreact", "/src/page.tsx")), true);
  assert.equal(_testing.shouldHandleSave(document("json", "/package.json")), true);
  assert.equal(_testing.shouldHandleSave(document("toml", "/twdoctor.toml")), true);
  assert.equal(_testing.shouldHandleSave(document("json", "/twdoctor-baseline.json")), true);
  assert.equal(_testing.shouldHandleSave(document("plaintext", "/notes.txt")), false);
});
