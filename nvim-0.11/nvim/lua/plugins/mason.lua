return {
  'mason-org/mason.nvim',
  build = ':MasonUpdate',
  cmd = {
    'Mason',
    'MasonInstall',
    'MasonUninstall',
    'MasonUninstallAll',
    'MasonLog',
  },
  opts = {
    -- Non-lsps are listed here
    ensure_installed = {
      'eslint_d',
      'prettierd',
      'stylua',
    },
  },
  config = function(_, opts)
    require('mason').setup(opts)
    local mr = require('mason-registry')

    mr.refresh(function()
      for _, tool in ipairs(opts.ensure_installed) do
        local p = mr.get_package(tool)
        if not p:is_installed() then
          p:install()
        end
      end
    end)
  end,
}
