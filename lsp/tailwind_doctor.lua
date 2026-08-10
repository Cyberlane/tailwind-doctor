return {
  cmd = { "tw-doctor", "lsp" },
  filetypes = {
    "astro",
    "css",
    "html",
    "javascript",
    "javascriptreact",
    "mdx",
    "svelte",
    "typescript",
    "typescriptreact",
    "vue",
  },
  root_markers = { "twdoctor.toml", "package.json", ".git" },
  flags = { debounce_text_changes = 120 },
}
