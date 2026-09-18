local util = require('lspconfig.util')

return {
  -- nvim-lspconfig does not yet recognize oxlint.config.mts as a root marker.
  -- This mirrors its upstream root_dir implementation with that marker added.
  root_dir = function(bufnr, on_dir)
    local fname = vim.api.nvim_buf_get_name(bufnr)
    local root_markers = util.insert_package_json(
      { '.oxlintrc.json', '.oxlintrc.jsonc', 'oxlint.config.ts', 'oxlint.config.mts' },
      { 'oxlint', 'vite%-plus' },
      fname
    )
    root_markers = util.root_markers_with_field(root_markers, { 'vite.config.ts' }, { 'vite%-plus', 'lint:' }, fname, 'all')
    on_dir(vim.fs.dirname(vim.fs.find(root_markers, { path = fname, upward = true })[1]))
  end,

  settings = {
    typeAware = false,
  },
}
