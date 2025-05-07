return {
  'NeogitOrg/neogit',
  dependencies = {
    'nvim-lua/plenary.nvim',
    'sindrets/diffview.nvim',
    'ibhagwan/fzf-lua',
  },
  keys = {
    {
      '<leader>gs',
      ':Neogit<CR>',
      mode = 'n',
      desc = 'Neo[g]it [s]how',
    },
  },
}
