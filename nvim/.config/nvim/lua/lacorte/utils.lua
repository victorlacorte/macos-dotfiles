local M = {}
local uv = vim.uv or vim.loop

-- Escape values for :substitute using a # delimiter.
-- The pattern is used with \V ("very nomagic"), so only delimiter and
-- backslash need special handling in the search text.
local escaped_names = function(old_name, new_name)
  return vim.fn.escape(old_name, [[\#]]), vim.fn.escape(new_name, [[\&#]])
end

-- Normalize any path to an absolute path.
local normalize_absolute_path = function(path)
  if not path or path == '' then
    return ''
  end

  return vim.fn.fnamemodify(path, ':p')
end

-- Resolve a quickfix item to a target buffer/path pair.
local resolve_qf_item_target = function(item)
  local target_bufnr
  local target_path = ''

  if item.bufnr and item.bufnr > 0 and vim.api.nvim_buf_is_valid(item.bufnr) then
    target_bufnr = item.bufnr
    target_path = vim.api.nvim_buf_get_name(target_bufnr)
  elseif item.filename and item.filename ~= '' then
    target_path = item.filename
  end

  return target_bufnr, target_path
end

-- Collect distinct target buffers from the quickfix list.
-- Side effect: referenced files are bufadd/bufload'ed and remain loaded.
local get_quickfix_target_bufnrs = function()
  local quickfix = vim.fn.getqflist({ items = 1 })
  local items = quickfix.items or {}
  local seen_files = {}
  local target_bufnrs = {}

  for _, item in ipairs(items) do
    local target_bufnr, target_path = resolve_qf_item_target(item)
    local absolute_path = normalize_absolute_path(target_path)

    if absolute_path ~= '' and uv.fs_stat(absolute_path) and not seen_files[absolute_path] then
      seen_files[absolute_path] = true

      if not target_bufnr then
        target_bufnr = vim.fn.bufadd(absolute_path)
      end

      if target_bufnr > 0 then
        vim.fn.bufload(target_bufnr)
        table.insert(target_bufnrs, target_bufnr)
      end
    end
  end

  return target_bufnrs
end

-- Apply the canonical whole-word substitute in one loaded buffer.
local substitute_word_in_buffer = function(bufnr, old_escaped, new_escaped)
  if not vim.api.nvim_buf_is_valid(bufnr) or not vim.api.nvim_buf_is_loaded(bufnr) then
    return
  end

  vim.api.nvim_buf_call(bufnr, function()
    vim.cmd(string.format('silent keepjumps %%s#\\V\\<%s\\>#%s#ge', old_escaped, new_escaped))
  end)
end

---@param old_name string
---@param new_name string
---@param bufnr? integer
function M.rename_current_buffer(old_name, new_name, bufnr)
  local old_escaped, new_escaped = escaped_names(old_name, new_name)
  local target_bufnr = bufnr or vim.api.nvim_get_current_buf()
  local view = vim.fn.winsaveview()

  substitute_word_in_buffer(target_bufnr, old_escaped, new_escaped)
  vim.fn.winrestview(view)
end

---@param old_name string
---@param new_name string
function M.rename_quickfix_buffers(old_name, new_name)
  local old_escaped, new_escaped = escaped_names(old_name, new_name)

  for _, bufnr in ipairs(get_quickfix_target_bufnrs()) do
    substitute_word_in_buffer(bufnr, old_escaped, new_escaped)
  end
end

return M
