return {
  'nvim-treesitter/nvim-treesitter',
  branch = 'main',
  lazy = false,
  build = ':TSUpdate',
  config = function()
    local install_dir = vim.fn.stdpath('data') .. '/site'
    vim.opt.runtimepath:append(install_dir)

    local ts = require('nvim-treesitter')
    ts.setup({ install_dir = install_dir })

    -- Parsers to install at startup (no-op if already present)
    local ensure_installed = {
      'bash',
      'json',
      'lua',
      'luadoc',
      'luap',
      'markdown',
      'markdown_inline',
      'vim',
      'vimdoc',
      'yaml',

      'css',
      'javascript',
      'jsdoc',
      'typescript',
      'tsx',

      'elixir',
      'heex',
      'eex',
    }

    ts.install(ensure_installed)

    vim.api.nvim_create_autocmd('FileType', {
      group = vim.api.nvim_create_augroup('treesitter-setup', { clear = true }),
      callback = function(ev)
        local buf = ev.buf
        local lang = vim.treesitter.language.get_lang(ev.match)
        if not lang then
          return
        end

        if not pcall(vim.treesitter.language.add, lang) then
          -- Parser not installed: trigger a non-blocking background install.
          -- Highlighting will work on the next buffer open for this filetype.
          ts.install({ lang })
          return
        end

        -- pcall guards against filetypes where language.add succeeds but no
        -- usable parser exists (e.g. fzf-lua's "fzf" filetype).
        if not pcall(vim.treesitter.start, buf, lang) then
          return
        end

        vim.bo[buf].indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
        vim.wo[0][0].foldmethod = 'expr'
        vim.wo[0][0].foldexpr = 'v:lua.vim.treesitter.foldexpr()'
        vim.wo[0][0].foldlevel = 99
      end,
    })
  end,
}
