return {
  init_options = {
    maxTsServerMemory = 8192,
    plugins = {
      -- https://github.com/styled-components/typescript-styled-plugin
      {
        name = '@styled/typescript-styled-plugin',
        location = vim.fn.expand(
          '$HOME/.volta/tools/image/packages/@styled/typescript-styled-plugin/lib/node_modules/@styled/typescript-styled-plugin/lib/index.js'
        ),
      },
    },
    preferences = {
      autoImportFileExcludePatterns = {
        'node_modules/**/internals',
        'node_modules/@mui/icons-material',
        'node_modules/@mui/lab',
        'node_modules/@mui/system',
        'node_modules/@mui/x-*/**',
        'node_modules/aws-sdk',
        'node_modules/framer-motion',
        'node_modules/typescript',
      },
    },
  },
  root_markers = { 'tsconfig.base.json', 'tsconfig.json', 'package.json', '.git' },
}
