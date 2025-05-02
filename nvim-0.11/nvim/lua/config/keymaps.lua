vim.keymap.set('n', '<leader>pv', function()
  vim.cmd('Explore')
end, { desc = 'Toggle Netrw' })

vim.keymap.set({ 'n', 'v' }, '<leader>f', function()
  vim.lsp.buf.format({
    filter = function(client)
      return client.name ~= 'ts_ls'
    end,
  })
end, { desc = '[F]ormat' })

-- Split window
vim.keymap.set('n', '<leader>ss', ':split<CR><C-w>w', { desc = '[S]plit horizontally' })
vim.keymap.set('n', '<leader>sv', ':vsplit<CR><C-w>w', { desc = '[S]plit [v]ertically' })
