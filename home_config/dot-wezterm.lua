local wezterm = require("wezterm")

local config = wezterm.config_builder()

local prev_workspace_name = nil

-- The filled in variant of the "<" symbol
local SOLID_LEFT_ARROW = utf8.char(0xe0b2)

-- Color palette for the backgrounds of each cell
local colors = {
	-- "#5c8374",
	-- "#183d3d",

	-- "#344955",
	-- "#50727b",

	"#2d4356",
	"#435b66",
}
-- Foreground color for the text across the fade
local text_fg = "#d0d0d0"

-- Solution adapted from https://github.com/wez/wezterm/discussions/4796
local sessionizer = function(window, pane)
	local projects = {}

	local success, stdout, stderr = wezterm.run_child_process({
		"/opt/homebrew/bin/fd",
		"--hidden",
		"--no-ignore",
		"--type=directory",
		"--max-depth=4",
		"--prune",
		"^.git$",
		"/Users/victorlacorte/First vault",
		"/Users/victorlacorte/coding",
	})

	if not success then
		wezterm.log_error("Failed to run fd: " .. stderr)
		return
	end

	for line in stdout:gmatch("([^\n]*)\n?") do
		local project = line:gsub("/.git/$", "")
		local label = project
		local id = project:gsub(".*/", "")

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
			title = "Select project",
			choices = projects,
		}),
		pane
	)
end

-- Font config
config.font = wezterm.font("JetBrains Mono")
config.font_size = 16
-- Disable ligatures: https://wezfurlong.org/wezterm/config/font-shaping.html#advanced-font-shaping-options
config.harfbuzz_features = { "calt=0", "clig=0", "liga=0" }

-- Tmux mappings
config.leader = { key = "a", mods = "CTRL", timeout_milliseconds = 1000 }
config.keys = {
	{
		key = "%",
		mods = "LEADER|SHIFT",
		action = wezterm.action.SplitHorizontal({ domain = "CurrentPaneDomain" }),
	},
	{
		key = '"',
		mods = "LEADER|SHIFT",
		action = wezterm.action.SplitVertical({ domain = "CurrentPaneDomain" }),
	},
	-- Send "CTRL-A" to the terminal when pressing CTRL-A, CTRL-A
	{
		key = "a",
		mods = "LEADER|CTRL",
		action = wezterm.action.SendKey({ key = "a", mods = "CTRL" }),
	},
	-- Create a new tab in the current workspace
	{ key = "c", mods = "LEADER", action = wezterm.action.SpawnTab("CurrentPaneDomain") },
	-- Switch to the next or previous tab
	{ key = "n", mods = "LEADER", action = wezterm.action.ActivateTabRelative(1) },
	{ key = "p", mods = "LEADER", action = wezterm.action.ActivateTabRelative(-1) },
	-- Show the launcher in fuzzy selection mode and have it list all
	-- workspaces and allow activating one.
	{
		key = "s",
		mods = "LEADER",
		action = wezterm.action_callback(function(window, pane)
			prev_workspace_name = window:active_workspace()

			window:perform_action(wezterm.action.ShowLauncherArgs({ flags = "FUZZY|WORKSPACES" }), pane)
		end),
	},
	-- Pane navigation
	{ key = "h", mods = "LEADER", action = wezterm.action.ActivatePaneDirection("Left") },
	{ key = "j", mods = "LEADER", action = wezterm.action.ActivatePaneDirection("Down") },
	{ key = "k", mods = "LEADER", action = wezterm.action.ActivatePaneDirection("Up") },
	{ key = "l", mods = "LEADER", action = wezterm.action.ActivatePaneDirection("Right") },
	{
		key = "L",
		mods = "LEADER|SHIFT",
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
		key = "Space",
		mods = "LEADER",
		action = wezterm.action_callback(sessionizer),
	},
}

wezterm.on("update-status", function(window)
	local date = wezterm.strftime("%Y-%m-%d %H:%M")
	-- https://wezfurlong.org/wezterm/config/lua/window/set_right_status.html
	window:set_right_status(wezterm.format({
		{ Foreground = { Color = colors[1] } },
		{ Text = SOLID_LEFT_ARROW },

		{ Foreground = { Color = text_fg } },
		{ Background = { Color = colors[1] } },
		{ Text = " " .. window:active_workspace() .. " " },
		{ Foreground = { Color = colors[2] } },
		{ Text = SOLID_LEFT_ARROW },

		{ Foreground = { Color = text_fg } },
		{ Background = { Color = colors[2] } },
		{ Text = " " .. date },
	}))
end)

return config
