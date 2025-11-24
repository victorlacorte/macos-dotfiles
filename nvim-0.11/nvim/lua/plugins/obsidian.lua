local notes_path = vim.fn.expand('$HOME/notes/')

return {
  'obsidian-nvim/obsidian.nvim',
  version = '*',
  lazy = true,
  ft = 'markdown',
  event = {
    'BufReadPre ' .. notes_path,
    'BufNewFile ' .. notes_path,
  },
  ---@module 'obsidian'
  ---@type obsidian.config
  opts = {
    completion = {
      blink = true,
      min_chars = 2,
      nvim_cmp = false,
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
