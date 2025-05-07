-- https://github.com/mantoni/eslint_d.js/issues/311
-- https://github.com/L2jLiga/nvim-none-ls-eslint_d/blob/master/lua/plugins/none-ls.lua
return {
  -- {
  --   'williamboman/mason.nvim',
  --   opts = function(_, opts)
  --     if type(opts.ensure_installed) == 'table' then
  --       vim.list_extend(opts.ensure_installed, { 'eslint_d' })
  --     end
  --   end,
  -- },
  {
    'nvimtools/none-ls.nvim',
    opts = function(_, opts)
      if type(opts.sources) == 'table' then
        vim.env.ESLINT_D_PPID = vim.fn.getpid()

        table.insert(opts.sources, require('none-ls.code_actions.eslint_d'))
        table.insert(opts.sources, require('none-ls.diagnostics.eslint_d'))
      end
    end,
  },
}
