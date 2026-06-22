return {
  filetypes = {
    'go',
    'gomod',
    'gosum',
    'gotmpl',
    'gowork',
  },
  settings = {
    gopls = {
      gofumpt = false,
      completeUnimported = true,
      staticcheck = true,
      usePlaceholders = true,
      templateExtensions = { 'tmpl', 'gotmpl' },
    },
  },
}
