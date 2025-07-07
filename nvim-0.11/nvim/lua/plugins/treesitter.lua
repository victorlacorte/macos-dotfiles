return {
  'nvim-treesitter/nvim-treesitter',
  build = ':TSUpdate',
  main = 'nvim-treesitter.configs', -- Sets main module to use for opts
  init = function()
    -- Folding: https://github.com/nvim-treesitter/nvim-treesitter#folding
    vim.wo.foldmethod = 'expr'
    vim.wo.foldexpr = 'v:lua.vim.treesitter.foldexpr()'
    vim.wo.foldenable = false
  end,
  opts = {
    ensure_installed = {
      'bash',
      'json',
      'jsonc',
      'lua',
      'luadoc',
      'luap',
      'markdown',
      'markdown_inline',
      'vim',
      'vimdoc',
      'yaml',

      'javascript',
      'jsdoc',
      'typescript',
      'tsx',

      'elixir',
      'heex',
      'eex',
    },
    auto_install = true,
    highlight = {
      enable = true,
      -- Some languages depend on vim's regex highlighting system (such as Ruby) for indent rules.
      --  If you are experiencing weird indenting issues, add the language to
      --  the list of additional_vim_regex_highlighting and disabled languages for indent.
      additional_vim_regex_highlighting = { 'ruby' },
    },
    indent = { enable = true, disable = { 'ruby' } },
  },
  config = function(_, opts)
    if type(opts.ensure_installed) == 'table' then
      local added = {}

      -- deduplicate entries
      opts.ensure_installed = vim.tbl_filter(function(lang)
        if added[lang] then
          return false
        end

        added[lang] = true

        return true
      end, opts.ensure_installed)
    end

    require('nvim-treesitter.configs').setup(opts)
  end,
}
