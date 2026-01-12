local wezterm = require('wezterm')

local config = wezterm.config_builder()

-- Variables to allow toggling between workspaces
-- `active_workspace` represents the workspace that was active the last time "update-status" ran
local active_workspace = {
  get = function()
    return wezterm.GLOBAL.active_workspace
  end,
  set = function(v)
    wezterm.GLOBAL.active_workspace = v
  end,
}

-- The workspace to return to
local prev_workspace = {
  get = function()
    return wezterm.GLOBAL.prev_workspace
  end,
  set = function(v)
    wezterm.GLOBAL.prev_workspace = v
  end,
}

-- Solution adapted from https://github.com/wez/wezterm/discussions/4796
local sessionizer = function(window, pane)
  local projects = {}

  local success, stdout, stderr = wezterm.run_child_process({
    '/opt/homebrew/bin/fd',
    '--hidden',
    '--no-ignore',
    '--type=directory',
    '--max-depth=3',
    '--prune',
    '^.git$',
    '/Users/victor',
    '/Users/victorlacorte/coding',
    '/Users/victorlacorte/notes',
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

local switch_to_prev_workspace = function(window, pane)
  local curr = window:active_workspace()
  local prev = prev_workspace.get()

  if prev == curr or prev == nil then
    return
  end

  window:perform_action(wezterm.action.SwitchToWorkspace({ name = prev }), pane)
end

-- https://wezterm.org/config/lua/config/front_end.html
-- NOTE: probably not necessary but I'm attempting to solve an issue with the
-- terminal in which all of a sudden it becomes sluggish and needs to be
-- restarted
config.front_end = 'OpenGL'

-- Colorscheme
-- https://wezfurlong.org/wezterm/colorschemes/t/index.html
config.color_scheme = 'tokyonight_night'

-- Font config
config.font = wezterm.font('JetBrains Mono')
config.font_size = 20
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
      switch_to_prev_workspace(window, pane)
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
local COLOR_ACCENT = '#c8d3f5'
local COLOR_DEFAULT_BG = '#2f334d'

local get_color = function(index)
  if index % 2 == 0 then
    return { fg = COLOR_ACCENT, bg = COLOR_DEFAULT_BG }
  end

  return { fg = COLOR_DEFAULT_BG, bg = COLOR_ACCENT }
end

wezterm.on('update-status', function(window)
  local curr_workspace = window:active_workspace()

  if active_workspace.get() ~= curr_workspace then
    prev_workspace.set(active_workspace.get())
    active_workspace.set(curr_workspace)
  end

  local cells = {
    curr_workspace,

    -- https://wezterm.org/config/lua/wezterm/strftime.html
    -- https://docs.rs/chrono/0.4.19/chrono/format/strftime/index.html
    wezterm.strftime('%d %b %H:%M'),
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
