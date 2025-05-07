return {
  'neovim/nvim-lspconfig',
  dependencies = {
    { 'mason-org/mason.nvim', version = '1.11.0', opts = {} },
    { 'mason-org/mason-lspconfig.nvim', version = '1.32.0' },
    'WhoIsSethDaniel/mason-tool-installer.nvim',

    { 'j-hui/fidget.nvim', opts = {} },

    'saghen/blink.cmp',
  },
  config = function()
    local servers = {
      eslint_d = {},
      prettierd = {},
      stylua = {},
      lua_ls = {
        -- Even though there are defined settings below, lazydev is effectively
        -- patching lua-language-server so I'm not sure these are even required.
        settings = {
          Lua = {
            completion = {
              callSnippet = 'Replace',
            },
            diagnostics = {
              globals = {
                'vim',
              },
            },
            format = {
              -- enabled via none-ls with stylua
              enable = false,
            },
            runtime = {
              version = 'LuaJIT',
            },
            telemetry = {
              enable = false,
            },
            workspace = {
              library = {
                vim.env.VIMRUNTIME,
                '${3rd}/luv/library',
              },
            },
          },
        },
      },
      -- https://github.com/neovim/nvim-lspconfig/blob/master/doc/configs.md#ts_ls
      ts_ls = {
        init_options = {
          maxTsServerMemory = 4096,
          plugins = {
            -- https://github.com/styled-components/typescript-styled-plugin
            {
              name = '@styled/typescript-styled-plugin',
              location = vim.fn.expand(
                '$HOME/.volta/tools/image/packages/@styled/typescript-styled-plugin/lib/node_modules/@styled/typescript-styled-plugin/lib/index.js'
              ),
            },
          },
        },
        root_markers = { 'tsconfig.base.json', 'tsconfig.json', 'package.json', '.git' },
      },
    }

    local ensure_installed = vim.tbl_keys(servers or {})
    require('mason-tool-installer').setup({ ensure_installed = ensure_installed })

    local function setup(server_name)
      local capabilities = vim.tbl_deep_extend('force', {}, vim.lsp.protocol.make_client_capabilities(), require('blink.cmp').get_lsp_capabilities())

      local server_opts = vim.tbl_deep_extend('force', {
        capabilities = vim.deepcopy(capabilities),
      }, servers[server_name] or {})

      require('lspconfig')[server_name].setup(server_opts)
    end

    require('mason-lspconfig').setup({
      automatic_installation = true,
      ensure_installed = {},
      handlers = { setup },
    })
  end,
  init = function()
    vim.api.nvim_create_autocmd('LspAttach', {
      group = vim.api.nvim_create_augroup('kickstart-lsp-attach', { clear = true }),
      callback = function(event)
        -- NOTE: Remember that Lua is a real programming language, and as such it is possible
        -- to define small helper and utility functions so you don't have to repeat yourself.
        --
        -- In this case, we create a function that lets us more easily define mappings specific
        -- for LSP related items. It sets the mode, buffer and description for us each time.
        local map = function(keys, func, desc, mode)
          mode = mode or 'n'
          vim.keymap.set(mode, keys, func, { buffer = event.buf, desc = 'LSP: ' .. desc })
        end

        local client = vim.lsp.get_client_by_id(event.data.client_id)

        -- Rename the variable under your cursor.
        --  Most Language Servers support renaming across files, etc.
        map('<leader>rn', vim.lsp.buf.rename, '[R]e[n]ame')

        -- Execute a code action, usually your cursor needs to be on top of an error
        -- or a suggestion from your LSP for this to activate.
        map('<leader>ca', vim.lsp.buf.code_action, '[G]oto Code [A]ction', { 'n', 'x' })

        -- Find references for the word under your cursor.
        map('<leader>gr', require('fzf-lua').lsp_references, '[G]oto [R]eferences')

        -- Jump to the definition of the word under your cursor.
        --  This is where a variable was first declared, or where a function is defined, etc.
        --  To jump back, press <C-t>.

        -- map('<leader>gd', client.name ~= 'ts_ls' and require('fzf-lua').lsp_definitions or function()
        --   local position_params = vim.lsp.util.make_position_params(0, 'utf-8')
        --
        --   client:exec_cmd({
        --     command = '_typescript.goToSourceDefinition',
        --     arguments = { vim.api.nvim_buf_get_name(0), position_params.position },
        --   })
        -- end, '[G]oto [D]efinition')

        map('<leader>gd', require('fzf-lua').lsp_definitions, '[G]oto [D]efinition')

        -- The following two autocommands are used to highlight references of the
        -- word under your cursor when your cursor rests there for a little while.
        --    See `:help CursorHold` for information about when this is executed
        --
        -- When you move your cursor, the highlights will be cleared (the second autocommand).
        if client and client:supports_method(vim.lsp.protocol.Methods.textDocument_documentHighlight, event.buf) then
          local highlight_augroup = vim.api.nvim_create_augroup('kickstart-lsp-highlight', { clear = false })
          vim.api.nvim_create_autocmd({ 'CursorHold', 'CursorHoldI' }, {
            buffer = event.buf,
            group = highlight_augroup,
            callback = vim.lsp.buf.document_highlight,
          })

          vim.api.nvim_create_autocmd({ 'CursorMoved', 'CursorMovedI' }, {
            buffer = event.buf,
            group = highlight_augroup,
            callback = vim.lsp.buf.clear_references,
          })

          vim.api.nvim_create_autocmd('LspDetach', {
            group = vim.api.nvim_create_augroup('kickstart-lsp-detach', { clear = true }),
            callback = function(event2)
              vim.lsp.buf.clear_references()
              vim.api.nvim_clear_autocmds({ group = 'kickstart-lsp-highlight', buffer = event2.buf })
            end,
          })
        end
      end,
    })

    vim.keymap.set({ 'n', 'v' }, '<leader>f', function()
      vim.lsp.buf.format({
        filter = function(client)
          return client.name ~= 'ts_ls'
        end,
      })
    end, { desc = '[F]ormat' })
  end,
}
