return {
  'MeanderingProgrammer/render-markdown.nvim',
  dependencies = {
    'nvim-treesitter/nvim-treesitter',
    'nvim-tree/nvim-web-devicons',
  },
  ---@module 'render-markdown'
  ---@type render.md.UserConfig
  opts = {
    enabled = false,
    -- https://github.com/olimorris/codecompanion.nvim/discussions/456#discussioncomment-11312434
    render_modes = true,
    sign = {
      enabled = false,
    },
  },
}
