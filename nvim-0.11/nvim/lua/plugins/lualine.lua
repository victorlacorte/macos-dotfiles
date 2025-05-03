-- https://github.com/nvim-lualine/lualine.nvim?tab=readme-ov-file#default-configuration
return {
  'nvim-lualine/lualine.nvim',
  dependencies = {
    'nvim-tree/nvim-web-devicons',
  },
  opts = {
    options = {
      theme = 'tokyonight-night',
    },
    -- remove these sections to improve readability on smaller screens
    sections = {
      lualine_x = {},
      lualine_y = {},
      lualine_z = {},
    },
  },
}
