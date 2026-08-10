# Editor Integration

Tailwind Doctor includes a Language Server Protocol endpoint, a dependency-free
VS Code client, and a native Neovim configuration. Diagnostics come from the same
analyzer and configuration as the CLI; editor integrations do not maintain a
second rule implementation.

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

## Neovim

Neovim 0.11 and newer can use the repository's native `vim.lsp` configuration.
Install the `tw-doctor` CLI on `PATH`, install this repository as a plugin, and
enable the configuration:

```lua
{
  "Cyberlane/tailwind-doctor",
  config = function()
    vim.lsp.enable("tailwind_doctor")
  end,
}
```

The example is a lazy.nvim plugin specification. Neovim's native package
mechanism is also supported; see [`editors/neovim`](../editors/neovim) for both
installation paths and binary-path overrides.

The configuration uses the same source filetypes and 120 ms change debounce as
the VS Code client. It selects the closest `twdoctor.toml`, `package.json`, or
Git root as the workspace. Run `:checkhealth vim.lsp` to verify attachment.
Restart the client after saving project-context files so the server rebuilds its
cached static configuration. On Neovim 0.11, call
`vim.lsp.enable("tailwind_doctor", false)` and then enable it again.

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
