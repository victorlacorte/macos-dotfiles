return {
  'nvimtools/none-ls.nvim',
  -- Copied from LazyVim/lua/lazyvim/util/plugin.lua
  event = { 'BufReadPost', 'BufNewFile', 'BufWritePre' },
  dependencies = {
    'mason.nvim',
    'nvimtools/none-ls-extras.nvim',
    'nvim-lua/plenary.nvim',
  },
  opts = function()
    local nls = require('null-ls')

    return {
      root_dir = require('null-ls.utils').root_pattern('.null-ls-root', '.neoconf.json', 'Makefile', '.git'),
      sources = {
        nls.builtins.formatting.prettierd.with({
          disabled_filetypes = { 'yaml' },
          prefer_local = 'node_modules/.bin',
        }),
        nls.builtins.formatting.stylua,
      },
    }
  end,
}
