# Tailwind Doctor for VS Code

This extension streams the current editor buffer to `tw-doctor lsp` and shows
the CLI's deterministic findings as native diagnostics. Analysis stays local,
does not execute project code, and does not make network requests.

Install the matching `tw-doctor` CLI release, then install the extension. If the
binary is not on `PATH`, set `tailwindDoctor.path` to its absolute path. The
extension supports multi-root workspaces and restarts each workspace server when
its settings change.

Tailwind Doctor is not affiliated with or endorsed by Tailwind Labs.
