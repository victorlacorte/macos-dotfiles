-- This plugin is cluttering code too much
return {
  'rafamadriz/friendly-snippets',
  ft = { 'markdown' },
  config = function()
    require('luasnip.loaders.from_vscode').lazy_load()
  end,
  -- This config is not working
  enabled = false
}
