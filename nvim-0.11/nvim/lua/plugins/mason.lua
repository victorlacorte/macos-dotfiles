return {
  'williamboman/mason.nvim',
  opts = {},
  build = ':MasonUpdate',
  init = function()
    local servers = {
      'lua-language-server',
      'typescript-language-server',
      'stylua',
    }

    local mr = require('mason-registry')

    local ensure_installed = function()
      for _, tool in ipairs(servers) do
        local p = mr.get_package(tool)
        if not p:is_installed() then
          p:install()
        end
      end
    end

    if mr.refresh then
      mr.refresh(ensure_installed)
    else
      ensure_installed()
    end
  end,
}
