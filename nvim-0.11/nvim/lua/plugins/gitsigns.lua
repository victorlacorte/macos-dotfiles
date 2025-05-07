return {
  -- Adds git related signs to the gutter, as well as utilities for managing changes
  'lewis6991/gitsigns.nvim',
  opts = {
    on_attach = function(bufnr)
      local gitsigns = require('gitsigns')

      vim.keymap.set('n', ']c', function()
        if vim.wo.diff then
          vim.cmd.normal({ ']c', bang = true })
        else
          gitsigns.nav_hunk('next')
        end
      end, { buffer = bufnr, desc = 'Next hunk' })

      vim.keymap.set('n', '[c', function()
        if vim.wo.diff then
          vim.cmd.normal({ '[c', bang = true })
        else
          gitsigns.nav_hunk('prev')
        end
      end, { buffer = bufnr, desc = 'Prev hunk' })

      vim.keymap.set('n', '<leader>ph', require('gitsigns').preview_hunk, { buffer = bufnr, desc = '[P]review [H]unk' })

      vim.keymap.set('n', '<leader>vh', function()
        gitsigns.setqflist('all')
      end, { buffer = bufnr, desc = '[V]iew hunks' })
    end,
  },
}
