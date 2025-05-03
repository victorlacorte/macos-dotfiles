return {
  'mbbill/undotree',
  opts = {},
  keys = {
    { '<leader>u', ':UndotreeToggle<CR>', desc = 'Toggle [U]ndoTree' },
  },
  init = function()
    vim.g.undotree_SetFocusWhenToggle = 1
  end,
}
