return {
  'mason-org/mason-lspconfig.nvim',
  -- event = 'BufReadPost',
  dependencies = {
    'mason-org/mason.nvim',
    'neovim/nvim-lspconfig',
    { 'j-hui/fidget.nvim', opts = {} },
    'saghen/blink.cmp',
  },
  opts = {
    ensure_installed = {
      'elixirls',
      'jsonls',
      'lua_ls',
      'tailwindcss',
      'ts_ls',
    },
  },
  init = function()
    vim.api.nvim_create_autocmd('LspAttach', {
      group = vim.api.nvim_create_augroup('kickstart-lsp-attach', { clear = true }),
      callback = function(event)
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
  end,
}
