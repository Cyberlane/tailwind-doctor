local source = debug.getinfo(1, "S").source:sub(2)
local repository_root = vim.fs.dirname(vim.fs.dirname(vim.fs.dirname(source)))
vim.opt.runtimepath:prepend(repository_root)

local config = vim.lsp.config.tailwind_doctor
assert(config, "tailwind_doctor LSP configuration was not discovered")
assert(vim.deep_equal(config.cmd, { "tw-doctor", "lsp" }), "unexpected server command")
assert(vim.deep_equal(config.root_markers, { "twdoctor.toml", "package.json", ".git" }), "unexpected root markers")
assert(config.flags.debounce_text_changes == 120, "unexpected change debounce")

local expected_filetypes = {
  astro = true,
  css = true,
  html = true,
  javascript = true,
  javascriptreact = true,
  mdx = true,
  svelte = true,
  typescript = true,
  typescriptreact = true,
  vue = true,
}
for _, filetype in ipairs(config.filetypes) do
  assert(expected_filetypes[filetype], "unexpected filetype: " .. filetype)
  expected_filetypes[filetype] = nil
end
assert(next(expected_filetypes) == nil, "a supported filetype is missing")

local binary = assert(vim.env.TW_DOCTOR_TEST_BINARY, "TW_DOCTOR_TEST_BINARY is not set")
local project_root = vim.fn.tempname()
assert(vim.fn.mkdir(project_root, "p") == 1, "could not create the test project")

local function cleanup()
  for _, client in ipairs(vim.lsp.get_clients({ name = "tailwind_doctor" })) do
    client:stop(true)
  end
  vim.fn.delete(project_root, "rf")
end

local success, failure = xpcall(function()
  vim.fn.writefile({ '{"dependencies":{"tailwindcss":"4.1.0"}}' }, project_root .. "/package.json")
  vim.fn.writefile({ '<div class="mt-[13px]"></div>' }, project_root .. "/page.html")

  vim.lsp.config("tailwind_doctor", { cmd = { binary, "lsp" } })
  vim.lsp.enable("tailwind_doctor")
  vim.cmd("filetype on")
  vim.cmd.edit(vim.fn.fnameescape(project_root .. "/page.html"))

  assert(vim.wait(5000, function()
    return #vim.lsp.get_clients({ name = "tailwind_doctor", bufnr = 0 }) == 1
  end), "tailwind_doctor did not attach")
  assert(vim.wait(5000, function()
    return #vim.diagnostic.get(0) > 0
  end), "tailwind_doctor did not publish diagnostics")

  local diagnostics = vim.diagnostic.get(0)
  assert(diagnostics[1].message:find("Avoid arbitrary values", 1, true), "unexpected diagnostic message")
end, debug.traceback)
cleanup()
if not success then
  vim.api.nvim_err_writeln(failure)
  vim.cmd.cquit()
end
