local M = {}

function M.is_git_repo()
  vim.fn.system('git rev-parse --is-inside-work-tree')

  return vim.v.shell_error == 0
end

local get_git_root = function()
  local dot_git_path = vim.fn.finddir('.git', '.;')

  return vim.fn.fnamemodify(dot_git_path, ':h')
end

function M.get_cwd()
  if M.is_git_repo() then
    return get_git_root()
  end

  return vim.fn.getcwd()
end

return M
