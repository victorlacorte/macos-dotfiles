local notes_path = vim.fn.expand('$HOME/notes/')

return {
  'obsidian-nvim/obsidian.nvim',
  version = '*',
  lazy = true,
  event = {
    'BufReadPre ' .. notes_path .. '*.md',
    'BufNewFile ' .. notes_path .. '*.md',
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
        path = notes_path,
      },
    },
  },
  -- config = function(_, opts)
  --   require('obsidian').setup(opts)
  --   vim.opt.conceallevel = 1
  -- end,
}
