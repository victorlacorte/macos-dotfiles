return {
  'ibhagwan/fzf-lua',
  dependencies = { 'nvim-tree/nvim-web-devicons' },
  opts = {
    keymap = {
      fzf = {
        ['ctrl-q'] = 'select-all+accept',
      },
    },
  },
  init = function()
    local fzf = require('fzf-lua')

    vim.keymap.set('n', '<leader><space>', fzf.buffers, { desc = '[ ] Find existing buffers' })
    vim.keymap.set('n', '<leader>/', fzf.lgrep_curbuf, { desc = '[/] Fuzzily search in current buffer' })
    vim.keymap.set('n', '<leader>sf', fzf.git_files, { desc = '[S]earch [F]iles' })
    vim.keymap.set('n', '<leader>sg', fzf.grep_project, { desc = '[S]earch by [G]rep' })
    vim.keymap.set('n', '<leader>sd', fzf.diagnostics_workspace, { desc = '[S]earch [D]iagnostics' })
    vim.keymap.set('n', '<leader>sw', fzf.grep_cword, { desc = '[S]earch current [W]ord' })
  end,
}
