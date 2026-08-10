# Editor Integration

Tailwind Doctor includes a Language Server Protocol endpoint and a dependency-free
VS Code client. Diagnostics come from the same analyzer and configuration as the
CLI; the editor integration does not maintain a second rule implementation.

## VS Code

1. Install the `tw-doctor` CLI matching the extension release and ensure it is
   on `PATH`.
2. Download `tailwind-doctor-vscode_VERSION.vsix` from the GitHub release.
3. In VS Code, run **Extensions: Install from VSIX...** and select the file.

Set `tailwindDoctor.path` when the binary is elsewhere. Use **Tailwind Doctor:
Restart Language Server** after changing project configuration. Multi-root
workspaces receive one isolated server per folder.

The extension sends full document text after a 120 ms debounce. This lets the
server analyze unsaved changes while keeping the project context — Tailwind
version, static theme inventory, prefix, ignore rules, and baseline — cached for
the session. Closing a document clears its diagnostics.
Saving `twdoctor.toml`, the baseline, `package.json`, a Tailwind config, or a CSS
file reloads static project context and republishes all open-document diagnostics
in URI order.

## Other editors

Run the server over standard input and output:

```bash
tw-doctor lsp
```

The server supports `initialize`, full-document `didOpen` and `didChange`,
`didSave`, `didClose`, `shutdown`, and `exit`. It advertises full text sync.
Diagnostics use zero-based UTF-16 LSP ranges, stable rule IDs as `code`, and
include confidence and score participation in `data`.

The server never writes source files, executes project code, makes network
requests, or emits protocol logging on stdout. Operational messages go to
stderr so the JSON-RPC stream stays valid.

Tailwind Doctor is not affiliated with or endorsed by Tailwind Labs.
