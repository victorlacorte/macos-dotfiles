local local_tsgo_build = '/Users/victor/coding/typescript-go/built/local/tsgo'

local function resolve_tsgo_cmd(root_dir)
  if root_dir then
    local tsgo = root_dir .. '/node_modules/.bin/tsgo'
    if vim.fn.executable(tsgo) == 1 then
      return { tsgo, '--lsp', '--stdio' }
    end
  end

  if vim.fn.executable(local_tsgo_build) == 1 then
    return { local_tsgo_build, '--lsp', '--stdio' }
  end

  return { 'tsgo', '--lsp', '--stdio' }
end

return {
  cmd = resolve_tsgo_cmd(vim.fs.root(0, { 'tsconfig.base.json', 'tsconfig.json', 'package.json', '.git' })),
  on_new_config = function(new_config, root_dir)
    new_config.cmd = resolve_tsgo_cmd(root_dir)
  end,
}
