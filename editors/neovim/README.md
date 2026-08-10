# Tailwind Doctor for Neovim

This repository includes a native LSP configuration for Neovim 0.11 or newer.
Install the `tw-doctor` CLI separately and ensure it is on `PATH`.

With lazy.nvim:

```lua
{
  "Cyberlane/tailwind-doctor",
  config = function()
    vim.lsp.enable("tailwind_doctor")
  end,
}
```

With Neovim's native package support:

```bash
git clone https://github.com/Cyberlane/tailwind-doctor.git \
  ~/.local/share/nvim/site/pack/tailwind-doctor/start/tailwind-doctor
```

Then add this to `init.lua`:

```lua
vim.lsp.enable("tailwind_doctor")
```

To use a binary that is not on `PATH`, override the command before enabling the
configuration:

```lua
vim.lsp.config("tailwind_doctor", {
  cmd = { "/path/to/tw-doctor", "lsp" },
})
vim.lsp.enable("tailwind_doctor")
```

Run `:checkhealth vim.lsp` to confirm that `tailwind_doctor` is enabled and
attached. Restart the client after changing `twdoctor.toml`, the baseline,
`package.json`, a Tailwind config, or a CSS file so the server reloads its cached
project context. On Neovim 0.11, disable and re-enable it:

```lua
vim.lsp.enable("tailwind_doctor", false)
vim.lsp.enable("tailwind_doctor")
```

Tailwind Doctor is not affiliated with or endorsed by Tailwind Labs.
