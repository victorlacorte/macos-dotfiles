return {
  'saghen/blink.cmp',
  version = '1.*',
  dependencies = {
    -- Snippet Engine
    {
      'L3MON4D3/LuaSnip',
      version = '2.*',
      build = (function()
        -- Build Step is needed for regex support in snippets.
        -- This step is not supported in many windows environments.
        -- Remove the below condition to re-enable on windows.
        if vim.fn.has('win32') == 1 or vim.fn.executable('make') == 0 then
          return
        end
        return 'make install_jsregexp'
      end)(),
      opts = {},
      dependencies = {
        'rafamadriz/friendly-snippets',
        -- 'MeanderingProgrammer/render-markdown.nvim',
      },
      config = function(_, opts)
        if opts then
          require('luasnip').config.setup(opts)
        end

        require('luasnip.loaders.from_vscode').lazy_load({
          -- This is better than `exclude` as JS-related snippets add too many suggestions
          include = {
            'lua',
            'markdown',
          },
        })

        -- require('render-markdown').setup({
        --   completions = { blink = { enabled = true } },
        -- })

        -- https://github.com/rafamadriz/friendly-snippets/wiki#extending-via-lazynvim
        -- require('luasnip').filetype_extend('typescript', { 'tsdoc' })
        -- require('luasnip').filetype_extend('javascript', { 'jsdoc' })
      end,
    },
    'folke/lazydev.nvim',
    'moyiz/blink-emoji.nvim',
  },
  --- @module 'blink.cmp'
  --- @type blink.cmp.Config
  opts = {
    keymap = {
      -- 'default' (recommended) for mappings similar to built-in completions
      --   <c-y> to accept ([y]es) the completion.
      --    This will auto-import if your LSP supports it.
      --    This will expand snippets if the LSP sent a snippet.
      -- 'super-tab' for tab to accept
      -- 'enter' for enter to accept
      -- 'none' for no mappings
      --
      -- For an understanding of why the 'default' preset is recommended,
      -- you will need to read `:help ins-completion`
      --
      -- No, but seriously. Please read `:help ins-completion`, it is really good!
      --
      -- All presets have the following mappings:
      -- <tab>/<s-tab>: move to right/left of your snippet expansion
      -- <c-space>: Open menu or open docs if already open
      -- <c-n>/<c-p> or <up>/<down>: Select next/previous item
      -- <c-e>: Hide menu
      -- <c-k>: Toggle signature help
      --
      -- See :h blink-cmp-config-keymap for defining your own keymap
      preset = 'default',

      -- For more advanced Luasnip keymaps (e.g. selecting choice nodes, expansion) see:
      --    https://github.com/L3MON4D3/LuaSnip?tab=readme-ov-file#keymaps
    },

    appearance = {
      -- 'mono' (default) for 'Nerd Font Mono' or 'normal' for 'Nerd Font'
      -- Adjusts spacing to ensure icons are aligned
      nerd_font_variant = 'mono',
    },

    completion = {
      -- By default, you may press `<c-space>` to show the documentation.
      -- Optionally, set `auto_show = true` to show the documentation after a delay.
      documentation = { auto_show = false, auto_show_delay_ms = 500 },
    },

    sources = {
      default = {
        'lsp',
        'buffer',
        'snippets',
        'path',
        'lazydev',
        'emoji',
        -- 'markdown',
      },
      providers = {
        lazydev = { module = 'lazydev.integrations.blink', score_offset = 100 },
        emoji = {
          module = 'blink-emoji',
          name = 'Emoji',
          score_offset = 15, -- Tune by preference
          opts = { insert = true }, -- Insert emoji (default) or complete its name
          should_show_items = function()
            return vim.tbl_contains(
              -- Enable emoji completion only for git commits and markdown.
              -- By default, enabled for all file-types.
              { 'gitcommit', 'markdown' },
              vim.o.filetype
            )
          end,
        },
        -- markdown = {
        --   name = 'RenderMarkdown',
        --   module = 'render-markdown.integ.blink',
        -- },
      },
    },

    snippets = { preset = 'luasnip' },

    -- Blink.cmp includes an optional, recommended rust fuzzy matcher,
    -- which automatically downloads a prebuilt binary when enabled.
    --
    -- By default, we use the Lua implementation instead, but you may enable
    -- the rust implementation via `'prefer_rust_with_warning'`
    --
    -- See :h blink-cmp-config-fuzzy for more information
    fuzzy = { implementation = 'lua' },

    -- Shows a signature help window while you type arguments for a function
    signature = { enabled = true },
  },
  opts_extend = { 'sources.default' },
}
