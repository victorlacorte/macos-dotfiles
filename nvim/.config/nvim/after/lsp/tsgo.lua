local function resolve_tsgo_cmd(root_dir)
  if root_dir then
    local local_tsc = root_dir .. '/node_modules/.bin/tsc'

    if vim.fn.executable(local_tsc) == 1 then
      return { local_tsc, '--lsp', '--stdio' }
    end
  end

  local global_tsc = vim.fn.exepath('tsc')

  if global_tsc ~= '' then
    return { global_tsc, '--lsp', '--stdio' }
  end

  return nil
end

return {
  cmd = resolve_tsgo_cmd(vim.fs.root(0, {
    'tsconfig.base.json',
    'tsconfig.json',
    'package.json',
    '.git',
  })),

  on_new_config = function(new_config, root_dir)
    new_config.cmd = resolve_tsgo_cmd(root_dir)
  end,
}
