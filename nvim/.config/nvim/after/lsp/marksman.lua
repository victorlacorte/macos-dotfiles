local notes_dir = vim.fs.normalize(vim.fn.expand('~/notes'))

local function is_in_notes(path)
  return path == notes_dir or vim.startswith(path, notes_dir .. '/')
end

return {
  root_dir = function(bufnr, on_dir)
    local filename = vim.fs.normalize(vim.api.nvim_buf_get_name(bufnr))

    if is_in_notes(filename) then
      return
    end

    on_dir(vim.fs.root(filename, { '.marksman.toml', '.git' }))
  end,
}
