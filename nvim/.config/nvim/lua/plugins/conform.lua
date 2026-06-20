-- https://github.com/stevearc/conform.nvim/blob/master/doc/recipes.md#lazy-loading-with-lazynvim
return {
  'stevearc/conform.nvim',
  event = { 'BufWritePre' },
  cmd = { 'ConformInfo' },
  keys = {
    {
      '<leader>f',
      function()
        require('conform').format({ async = true })
      end,
      mode = { 'n', 'v' },
      desc = '[F]ormat buffer',
    },
  },
  -- This will provide type hinting with LuaLS
  ---@module "conform"
  ---@type conform.setupOpts
  opts = {
    formatters_by_ft = {
      go = { 'gofmt' },
      lua = { 'stylua' },
      markdown = { 'oxfmt', 'injected' },
      json = { 'jq' },

      javascript = { 'oxfmt' },
      javascriptreact = { 'oxfmt' },
      typescript = { 'oxfmt' },
      typescriptreact = { 'oxfmt' },

      sh = { 'shfmt' },
      bash = { 'shfmt' },
    },
    default_format_opts = {
      lsp_format = 'fallback',
    },
    -- Customize formatters
    formatters = {
      oxfmt = {
        cwd = function(_, ctx)
          return vim.fs.root(ctx.dirname, { '.oxfmtrc.json' })
        end,
      },
      shfmt = {
        prepend_args = { '-i', '2', '-ci', '-bn' },
      },
    },
  },
  -- init = function()
  --   -- If you want the formatexpr, here is the place to set it
  --   vim.o.formatexpr = "v:lua.require'conform'.formatexpr()"
  -- end,
}
