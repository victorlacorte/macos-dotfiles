-- https://github.com/neovim/nvim-lspconfig/blob/master/doc/configs.md#ts_ls
return {
  init_options = {
    plugins = {
      -- https://github.com/styled-components/typescript-styled-plugin
      {
        name = '@styled/typescript-styled-plugin',
        location = vim.fn.expand('$HOME/n/lib/node_modules/@styled/typescript-styled-plugin/lib/index.js'),
      },
    },
  },
}
