local wezterm = require('wezterm')

local config = wezterm.config_builder()

local prev_workspace_name = nil

-- Solution adapted from https://github.com/wez/wezterm/discussions/4796
local sessionizer = function(window, pane)
  local projects = {}

  local success, stdout, stderr = wezterm.run_child_process({
    '/opt/homebrew/bin/fd',
    '--hidden',
    '--no-ignore',
    '--type=directory',
    '--max-depth=4',
    '--prune',
    '^.git$',
    '/Users/victor',
    '/Users/victorlacorte',
  })

  if not success then
    wezterm.log_error('Failed to run fd: ' .. stderr)
    return
  end

  for line in stdout:gmatch('([^\n]*)\n?') do
    local project = line:gsub('/.git/$', '')
    local label = project
    local id = project:gsub('.*/', '')

    table.insert(projects, { label = tostring(label), id = tostring(id) })
  end

  window:perform_action(
    wezterm.action.InputSelector({
      action = wezterm.action_callback(function(win, _, id, label)
        if not id and not label then
          return
        end

        win:perform_action(wezterm.action.SwitchToWorkspace({ name = id, spawn = { cwd = label } }), pane)
      end),
      fuzzy = true,
      title = 'Select project',
      choices = projects,
    }),
    pane
  )
end

-- Colorscheme
-- https://wezfurlong.org/wezterm/colorschemes/t/index.html
config.color_scheme = 'tokyonight_night'

-- Font config
config.font = wezterm.font('JetBrains Mono')
config.font_size = 16
-- Disable ligatures: https://wezfurlong.org/wezterm/config/font-shaping.html#advanced-font-shaping-options
config.harfbuzz_features = { 'calt=0', 'clig=0', 'liga=0' }

config.leader = { key = 'a', mods = 'CTRL', timeout_milliseconds = 1000 }

-- https://wezfurlong.org/wezterm/config/lua/wezterm.gui/default_key_tables.html
-- Override certain key assignments without defining the entire copy_mode key table
local copy_mode = wezterm.gui.default_key_tables().copy_mode

table.insert(copy_mode, {
  key = 'Enter',
  mods = 'NONE',
  action = wezterm.action.Multiple({
    { CopyTo = 'ClipboardAndPrimarySelection' },
    { CopyMode = 'Close' },
  }),
})

config.key_tables = {
  copy_mode = copy_mode,
}

config.keys = {
  {
    key = '[',
    mods = 'LEADER',
    action = wezterm.action.ActivateCopyMode,
  },
  {
    key = '%',
    mods = 'LEADER|SHIFT',
    action = wezterm.action.SplitHorizontal({ domain = 'CurrentPaneDomain' }),
  },
  {
    key = '"',
    mods = 'LEADER|SHIFT',
    action = wezterm.action.SplitVertical({ domain = 'CurrentPaneDomain' }),
  },
  -- Send "CTRL-A" to the terminal when pressing CTRL-A, CTRL-A
  {
    key = 'a',
    mods = 'LEADER|CTRL',
    action = wezterm.action.SendKey({ key = 'a', mods = 'CTRL' }),
  },
  -- Create a new tab in the current workspace
  { key = 'c', mods = 'LEADER', action = wezterm.action.SpawnTab('CurrentPaneDomain') },
  -- Switch to the next or previous tab
  { key = 'n', mods = 'LEADER', action = wezterm.action.ActivateTabRelative(1) },
  { key = 'p', mods = 'LEADER', action = wezterm.action.ActivateTabRelative(-1) },
  -- Show the launcher in fuzzy selection mode and have it list all
  -- workspaces and allow activating one.
  {
    key = 's',
    mods = 'LEADER',
    action = wezterm.action_callback(function(window, pane)
      prev_workspace_name = window:active_workspace()

      window:perform_action(wezterm.action.ShowLauncherArgs({ flags = 'FUZZY|WORKSPACES' }), pane)
    end),
  },
  -- Pane navigation
  { key = 'h', mods = 'LEADER', action = wezterm.action.ActivatePaneDirection('Left') },
  { key = 'j', mods = 'LEADER', action = wezterm.action.ActivatePaneDirection('Down') },
  { key = 'k', mods = 'LEADER', action = wezterm.action.ActivatePaneDirection('Up') },
  { key = 'l', mods = 'LEADER', action = wezterm.action.ActivatePaneDirection('Right') },
  {
    key = 'L',
    mods = 'LEADER|SHIFT',
    action = wezterm.action_callback(function(window, pane)
      local curr_workspace_name = window:active_workspace()

      if prev_workspace_name == nil or curr_workspace_name == prev_workspace_name then
        return
      end

      window:perform_action(wezterm.action.SwitchToWorkspace({ name = prev_workspace_name }), pane)

      prev_workspace_name = curr_workspace_name
    end),
  },

  -- WARN: still didnt get the close workspace functionality to work, so CTRL-d will have to do for now
  --{
  --	key = "w",
  --	mods = "LEADER",
  --	action = wezterm.action_callback(function(window)
  --		for tab in pairs(window:tabs()) do
  --		-- close each tab individually
  --		end
  --	end),
  --},
  {
    key = 'Space',
    mods = 'LEADER',
    action = wezterm.action_callback(sessionizer),
  },
}

-- The filled in variant of the "<" symbol
local SOLID_LEFT_ARROW = wezterm.nerdfonts.pl_right_hard_divider

-- https://github.com/folke/tokyonight.nvim/blob/main/lua/tokyonight/colors/moon.lua
-- blue
local COLOR_BLUE = '#82aaff'
-- bg_highlight
local COLOR_BLACK = '#2f334d'

local get_color = function(index)
  if index % 2 == 0 then
    return { fg = COLOR_BLUE, bg = COLOR_BLACK }
  end

  return { fg = COLOR_BLACK, bg = COLOR_BLUE }
end

wezterm.on('update-status', function(window)
  local cells = {
    window:active_workspace(),
    wezterm.strftime('%Y-%m-%d %H:%M'),
  }

  -- An entry for each battery (typically 0 or 1)
  for _, b in ipairs(wezterm.battery_info()) do
    table.insert(cells, '⚡ ' .. string.format('%.0f%%', b.state_of_charge * 100))
  end

  -- The elements to be formatted
  local elements = {}
  local num_fmt_cells = 0

  local push = function(text)
    local cell_index = num_fmt_cells + 1

    table.insert(elements, { Foreground = { Color = get_color(cell_index).bg } })
    table.insert(elements, { Background = { Color = get_color(cell_index).fg } })
    table.insert(elements, { Text = SOLID_LEFT_ARROW })
    table.insert(elements, { Foreground = { Color = get_color(cell_index).fg } })
    table.insert(elements, { Background = { Color = get_color(cell_index).bg } })
    table.insert(elements, { Text = ' ' .. text .. ' ' })

    num_fmt_cells = num_fmt_cells + 1
  end

  while #cells > 0 do
    local cell = table.remove(cells, 1)

    push(cell)
  end

  -- https://wezfurlong.org/wezterm/config/lua/window/set_right_status.html
  window:set_right_status(wezterm.format(elements))
end)

return config
