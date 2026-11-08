-- Allow moving highlighted content up or down
vim.keymap.set('v', 'J', ":m '>+1<CR>gv=gv")
vim.keymap.set('v', 'K', ":m '<-2<CR>gv=gv")

-- Split window
vim.keymap.set('n', '<leader>ss', ':split<CR><C-w>w', { desc = '[S]plit horizontally' })
vim.keymap.set('n', '<leader>sv', ':vsplit<CR><C-w>w', { desc = '[S]plit [v]ertically' })

vim.keymap.set('n', '<leader>vd', vim.diagnostic.open_float, { desc = '[V]iew [d]iagnostic' })

-- Copy the current file relative path to the Git repo's root
-- Essentially a smarter way of `:!ls %:p | pbcopy`
vim.keymap.set('n', '<leader>yp', function()
  local root = vim.fs.root(0, '.git')
  local path = vim.api.nvim_buf_get_name(0)

  if not root or path == '' then
    vim.notify('Current file is not in a Git repository', vim.log.levels.WARN)
    return
  end

  local relative = vim.fs.relpath(root, path)
  if relative then
    vim.fn.setreg('+', relative)
  end
end, { desc = 'Copy repo-relative path' })

-- Navigate the quickfix's content more easily
vim.keymap.set('n', '<C-j>', '<cmd>cnext<CR>zz')
vim.keymap.set('n', '<C-k>', '<cmd>cprev<CR>zz')

-- Remap for dealing with word wrap
vim.keymap.set('n', 'k', "v:count == 0 ? 'gk' : 'k'", { expr = true, silent = true })
vim.keymap.set('n', 'j', "v:count == 0 ? 'gj' : 'j'", { expr = true, silent = true })

-- Don't yank with x
vim.keymap.set('n', 'x', '"_x')
