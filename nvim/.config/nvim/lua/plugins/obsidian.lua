local notes_dir = vim.fs.normalize(vim.fn.expand('~/notes'))

return {
  'obsidian-nvim/obsidian.nvim',
  version = '*',
  lazy = true,
  event = {
    'BufReadPre ' .. notes_dir .. '*.md',
    'BufNewFile ' .. notes_dir .. '*.md',
  },
  ---@module 'obsidian'
  ---@type obsidian.config
  opts = {
    completion = {
      min_chars = 2,
    },
    legacy_commands = false,
    workspaces = {
      {
        name = 'notes',
        path = notes_dir,
      },
    },
    daily_notes = {
      folder = 'dailies',
    },
  },
  -- config = function(_, opts)
  --   require('obsidian').setup(opts)
  --   vim.opt.conceallevel = 1
  -- end,
}
