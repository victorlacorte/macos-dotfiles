-- Allow moving highlighted content up or down
vim.keymap.set('v', 'J', ":m '>+1<CR>gv=gv")
vim.keymap.set('v', 'K', ":m '<-2<CR>gv=gv")

vim.keymap.set('n', '<leader>pv', function()
  vim.cmd('Explore')
end, { desc = 'Toggle Netrw' })

-- Split window
vim.keymap.set('n', '<leader>ss', ':split<CR><C-w>w', { desc = '[S]plit horizontally' })
vim.keymap.set('n', '<leader>sv', ':vsplit<CR><C-w>w', { desc = '[S]plit [v]ertically' })

vim.keymap.set('n', '<leader>vd', vim.diagnostic.open_float, { desc = '[V]iew [d]iagnostic' })
