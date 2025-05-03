return {
  -- Even though there are defined settings below, lazydev is effectively
  -- patching lua-language-server so I'm not sure these are even required.
  settings = {
    Lua = {
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
}
