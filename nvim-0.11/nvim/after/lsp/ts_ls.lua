return {
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
}
