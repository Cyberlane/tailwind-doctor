"use strict";

const childProcess = require("child_process");
const vscode = require("vscode");

const supportedLanguages = new Set([
  "astro", "css", "html", "javascript", "javascriptreact", "mdx",
  "svelte", "typescript", "typescriptreact", "vue",
]);

class DoctorClient {
  constructor(folder, diagnostics, output) {
    this.folder = folder;
    this.diagnostics = diagnostics;
    this.output = output;
    this.sequence = 0;
    this.pending = new Map();
    this.buffer = Buffer.alloc(0);
    this.debounce = new Map();
    this.ready = false;
    this.stopping = false;
  }

  start() {
    const configuration = vscode.workspace.getConfiguration("tailwindDoctor", this.folder.uri);
    const executable = configuration.get("path", "tw-doctor");
    this.trace = configuration.get("trace.server", false);
    this.process = childProcess.spawn(executable, ["lsp"], {
      cwd: this.folder.uri.fsPath,
      stdio: ["pipe", "pipe", "pipe"],
      windowsHide: true,
    });
    this.process.stdout.on("data", (chunk) => this.receive(chunk));
    this.process.stderr.on("data", (chunk) => this.output.append(chunk.toString()));
    this.process.on("error", (error) => {
      this.failPending(error);
      this.output.appendLine(`Could not start ${executable}: ${error.message}`);
      vscode.window.showErrorMessage(`Tailwind Doctor could not start: ${error.message}`);
    });
    this.process.on("exit", (code, signal) => {
      this.failPending(new Error(`language server exited with code ${code}`));
      this.ready = false;
      if (!this.stopping) {
        this.output.appendLine(`Language server exited (code ${code}, signal ${signal || "none"}).`);
      }
    });
    return this.request("initialize", {
      processId: process.pid,
      rootUri: this.folder.uri.toString(),
      workspaceFolders: [{ uri: this.folder.uri.toString(), name: this.folder.name }],
      capabilities: {},
      clientInfo: { name: "Tailwind Doctor for VS Code" },
    }).then(() => {
      this.ready = true;
      this.notify("initialized", {});
      for (const document of vscode.workspace.textDocuments) {
        if (this.owns(document) && supportedLanguages.has(document.languageId)) {
          this.open(document);
        }
      }
    }).catch((error) => {
      this.output.appendLine(`Initialization failed: ${error.message}`);
      vscode.window.showErrorMessage(`Tailwind Doctor initialization failed: ${error.message}`);
    });
  }

  owns(document) {
    const folder = vscode.workspace.getWorkspaceFolder(document.uri);
    return folder && folder.uri.toString() === this.folder.uri.toString();
  }

  open(document) {
    if (!this.ready) return;
    this.notify("textDocument/didOpen", {
      textDocument: {
        uri: document.uri.toString(), languageId: document.languageId,
        version: document.version, text: document.getText(),
      },
    });
  }

  change(document) {
    if (!this.ready) return;
    const key = document.uri.toString();
    clearTimeout(this.debounce.get(key));
    this.debounce.set(key, setTimeout(() => {
      this.debounce.delete(key);
      this.notify("textDocument/didChange", {
        textDocument: { uri: key, version: document.version },
        contentChanges: [{ text: document.getText() }],
      });
    }, 120));
  }

  save(document) {
    if (!this.ready) return;
    this.notify("textDocument/didSave", {
      textDocument: { uri: document.uri.toString() }, text: document.getText(),
    });
    if (isProjectContextDocument(document)) {
      this.notify("workspace/didChangeWatchedFiles", {
        changes: [{ uri: document.uri.toString(), type: 2 }],
      });
    }
  }

  close(document) {
    if (!this.ready) return;
    this.notify("textDocument/didClose", { textDocument: { uri: document.uri.toString() } });
  }

  request(method, params) {
    const id = ++this.sequence;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.send({ jsonrpc: "2.0", id, method, params });
    });
  }

  notify(method, params) {
    this.send({ jsonrpc: "2.0", method, params });
  }

  send(message) {
    if (!this.process || !this.process.stdin.writable) return;
    const body = Buffer.from(JSON.stringify(message));
    if (this.trace) this.output.appendLine(`--> ${body.toString()}`);
    this.process.stdin.write(`Content-Length: ${body.length}\r\n\r\n`);
    this.process.stdin.write(body);
  }

  receive(chunk) {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    while (true) {
      const headerEnd = this.buffer.indexOf("\r\n\r\n");
      if (headerEnd < 0) return;
      const header = this.buffer.subarray(0, headerEnd).toString();
      const match = /(?:^|\r\n)Content-Length:\s*(\d+)/i.exec(header);
      if (!match) {
        this.output.appendLine("Language server sent a frame without Content-Length.");
        this.buffer = Buffer.alloc(0);
        return;
      }
      const length = Number(match[1]);
      const bodyStart = headerEnd + 4;
      if (this.buffer.length < bodyStart + length) return;
      const body = this.buffer.subarray(bodyStart, bodyStart + length).toString();
      this.buffer = this.buffer.subarray(bodyStart + length);
      if (this.trace) this.output.appendLine(`<-- ${body}`);
      try {
        this.handle(JSON.parse(body));
      } catch (error) {
        this.output.appendLine(`Could not decode language-server message: ${error.message}`);
      }
    }
  }

  handle(message) {
    if (message.id !== undefined) {
      const pending = this.pending.get(message.id);
      if (!pending) return;
      this.pending.delete(message.id);
      if (message.error) pending.reject(new Error(message.error.message));
      else pending.resolve(message.result);
      return;
    }
    if (message.method === "window/showMessage") {
      this.output.appendLine(message.params.message);
      vscode.window.showErrorMessage(message.params.message);
      return;
    }
    if (message.method !== "textDocument/publishDiagnostics") return;
    const uri = vscode.Uri.parse(message.params.uri);
    const diagnostics = message.params.diagnostics.map((entry) => {
      const start = new vscode.Position(entry.range.start.line, entry.range.start.character);
      const end = new vscode.Position(entry.range.end.line, entry.range.end.character);
      const severities = [
        vscode.DiagnosticSeverity.Information,
        vscode.DiagnosticSeverity.Error,
        vscode.DiagnosticSeverity.Warning,
        vscode.DiagnosticSeverity.Information,
        vscode.DiagnosticSeverity.Hint,
      ];
      const diagnostic = new vscode.Diagnostic(
        new vscode.Range(start, end), entry.message,
        severities[entry.severity] || vscode.DiagnosticSeverity.Information,
      );
      diagnostic.code = entry.code;
      diagnostic.source = entry.source || "tw-doctor";
      return diagnostic;
    });
    this.diagnostics.set(uri, diagnostics);
  }

  failPending(error) {
    for (const pending of this.pending.values()) pending.reject(error);
    this.pending.clear();
  }

  async stop() {
    this.stopping = true;
    for (const timeout of this.debounce.values()) clearTimeout(timeout);
    this.debounce.clear();
    if (!this.process || this.process.killed) return;
    try {
      if (this.ready) await Promise.race([
        this.request("shutdown", null),
        new Promise((resolve) => setTimeout(resolve, 500)),
      ]);
      this.notify("exit", null);
    } finally {
      setTimeout(() => {
        if (this.process && !this.process.killed) this.process.kill();
      }, 500);
    }
  }
}

function isProjectContextDocument(document) {
  const name = document.uri.path.split("/").pop() || "";
  return document.languageId === "css" || name === "package.json" ||
    name === "twdoctor.toml" || name === "twdoctor-baseline.json" ||
    name.startsWith("tailwind.config.");
}

function shouldHandleSave(document) {
  return supportedLanguages.has(document.languageId) || isProjectContextDocument(document);
}

function activate(context) {
  const output = vscode.window.createOutputChannel("Tailwind Doctor");
  const diagnostics = vscode.languages.createDiagnosticCollection("tw-doctor");
  const clients = new Map();

  const start = () => {
    for (const folder of vscode.workspace.workspaceFolders || []) {
      const key = folder.uri.toString();
      if (clients.has(key)) continue;
      const client = new DoctorClient(folder, diagnostics, output);
      clients.set(key, client);
      client.start();
    }
  };

  const stop = async () => {
    await Promise.all([...clients.values()].map((client) => client.stop()));
    clients.clear();
    diagnostics.clear();
  };

  const clientFor = (document) => {
    const folder = vscode.workspace.getWorkspaceFolder(document.uri);
    return folder && clients.get(folder.uri.toString());
  };

  context.subscriptions.push(
    output,
    diagnostics,
    vscode.workspace.onDidOpenTextDocument((document) => {
      if (supportedLanguages.has(document.languageId)) clientFor(document)?.open(document);
    }),
    vscode.workspace.onDidChangeTextDocument((event) => {
      if (supportedLanguages.has(event.document.languageId)) clientFor(event.document)?.change(event.document);
    }),
    vscode.workspace.onDidSaveTextDocument((document) => {
      if (shouldHandleSave(document)) {
        clientFor(document)?.save(document);
      }
    }),
    vscode.workspace.onDidCloseTextDocument((document) => clientFor(document)?.close(document)),
    vscode.workspace.onDidChangeWorkspaceFolders(async () => { await stop(); start(); }),
    vscode.workspace.onDidChangeConfiguration(async (event) => {
      if (event.affectsConfiguration("tailwindDoctor")) { await stop(); start(); }
    }),
    vscode.commands.registerCommand("tailwindDoctor.restart", async () => { await stop(); start(); }),
    { dispose: () => { void stop(); } },
  );
  start();
}

function deactivate() {}

module.exports = { activate, deactivate, _testing: { shouldHandleSave } };
