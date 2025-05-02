return {
  'neovim/nvim-lspconfig',
  dependencies = {
    'williamboman/mason.nvim',
    'williamboman/mason-lspconfig.nvim',
    'saghen/blink.cmp',
  },
  config = function()
    -- vim.api.nvim_create_autocmd('LspAttach', {
    --   callback = function(ev)
    --     local client = vim.lsp.get_client_by_id(ev.data.client_id)
    --
    --     if client and client:supports_method('textDocument/completion') then
    --       vim.lsp.completion.enable(true, client.id, ev.buf, { autotrigger = true })
    --     end
    --   end,
    -- })

    local capabilities = require('blink.cmp').get_lsp_capabilities()

    local servers = {
      'lua_ls',
      'ts_ls',
    }

    require('mason-lspconfig').setup({
      -- ensure_installed = {},
      -- automatic_installation = false,
      handlers = {
        function(server_name)
          local server = servers[server_name] or {}
          -- This handles overriding only values explicitly passed
          -- by the server configuration above. Useful when disabling
          -- certain features of an LSP (for example, turning off formatting for ts_ls)
          server.capabilities = vim.tbl_deep_extend('force', {}, capabilities, server.capabilities or {})

          require('lspconfig')[server_name].setup(server)
        end,
      },
    })

    vim.lsp.enable(servers)
  end,
}
