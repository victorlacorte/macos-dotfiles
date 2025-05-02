return {
  'ibhagwan/fzf-lua',
  -- optional for icon support
  dependencies = { 'nvim-tree/nvim-web-devicons' },
  -- or if using mini.icons/mini.nvim
  -- dependencies = { "echasnovski/mini.icons" },
  opts = {},
  config = function(_, opts)
    local fzf = require('fzf-lua')

    fzf.setup(opts)

    vim.keymap.set('n', '<leader><space>', fzf.buffers, { desc = '[ ] Find existing buffers' })
    vim.keymap.set('n', '<leader>sf', fzf.files, { desc = '[S]earch [F]iles' })
    -- not sure what is the actual difference between live_grep_native and live_grep
    vim.keymap.set('n', '<leader>sg', fzf.live_grep_native, { desc = '[S]earch by [G]rep' })
    vim.keymap.set('n', '<leader>sd', fzf.diagnostics_workspace, {})

    -- vim.keymap.set("n", "<leader>fg", fzf.git_files, {})
    -- vim.keymap.set("n", "<leader>fw", fzf.grep_cword, {})
    -- vim.keymap.set("v", "<leader>fw", fzf.grep_visual, {})
    -- vim.keymap.set("n", "<leader>fW", fzf.grep_cWORD, {})
    -- vim.keymap.set("n", "<leader>fp", fzf.live_grep, {})
    -- vim.keymap.set("n", "<leader>fP", fzf.live_grep_native, {})
    -- vim.keymap.set("n", "<leader>K", fzf.keymaps)
  end,
}
