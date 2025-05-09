-- Allow moving highlighted content up or down
vim.keymap.set('v', 'J', ":m '>+1<CR>gv=gv")
vim.keymap.set('v', 'K', ":m '<-2<CR>gv=gv")

-- Split window
vim.keymap.set('n', '<leader>ss', ':split<CR><C-w>w', { desc = '[S]plit horizontally' })
vim.keymap.set('n', '<leader>sv', ':vsplit<CR><C-w>w', { desc = '[S]plit [v]ertically' })

vim.keymap.set('n', '<leader>vd', vim.diagnostic.open_float, { desc = '[V]iew [d]iagnostic' })

-- Navigate the quickfix's content more easily
vim.keymap.set('n', '<C-j>', '<cmd>cnext<CR>zz')
vim.keymap.set('n', '<C-k>', '<cmd>cprev<CR>zz')

-- Remap for dealing with word wrap
vim.keymap.set('n', 'k', "v:count == 0 ? 'gk' : 'k'", { expr = true, silent = true })
vim.keymap.set('n', 'j', "v:count == 0 ? 'gj' : 'j'", { expr = true, silent = true })
