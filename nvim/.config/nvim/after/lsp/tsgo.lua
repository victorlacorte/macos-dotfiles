return {
  cmd = function(dispatchers, config)
    local root_dir = config.root_dir
    local candidates = {
      vim.fs.joinpath(root_dir, 'node_modules/typescript-7/bin/tsc'),
      vim.fs.joinpath(root_dir, 'node_modules/.bin/tsc'),
    }

    for _, candidate in ipairs(candidates) do
      if vim.fn.executable(candidate) == 1 then
        return vim.lsp.rpc.start({ candidate, '--lsp', '--stdio' }, dispatchers, {
          cwd = config.cmd_cwd,
          env = config.cmd_env,
          detached = config.detached,
        })
      end
    end

    error(('No local TypeScript executable found for tsgo workspace %q'):format(root_dir))
  end,
}
