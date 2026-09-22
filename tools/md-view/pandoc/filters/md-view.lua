-- Presentation-only transformations for md-view.
-- Pandoc remains responsible for parsing Markdown and writing HTML.

local function attr_value(attr, name)
  if not attr or not attr.attributes then
    return nil
  end
  return attr.attributes[name]
end

local function safe_attribute_name(name)
  return name:match("^data%-[%w_.:%%-]+$")
      or name:match("^aria%-[%w_.:%%-]+$")
      or name == "role"
      or name == "title"
      or name == "lang"
      or name == "dir"
end

local alert_titles = {
  note = true,
  tip = true,
  important = true,
  warning = true,
  caution = true,
}

local alert_icon_paths = {
  note = '<path d="M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"/>',
  tip = '<path d="M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.689.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.218-.292.466-.584.689-.848l.214-.253c.56-.679.984-1.32.984-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z"/>',
  important = '<path d="M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"/>',
  warning = '<path d="M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"/>',
  caution = '<path d="M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.14.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"/>',
}

local function has_class(el, name)
  for _, class_name in ipairs(el.classes) do
    if class_name == name then
      return true
    end
  end
  return false
end

local function alert_kind(classes)
  local kind = nil
  for _, class_name in ipairs(classes) do
    if alert_titles[class_name] then
      if kind and kind ~= class_name then
        return nil
      end
      kind = class_name
    end
  end
  return kind
end

local function alert_icon(kind)
  return string.format(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" fill="currentColor" class="markdown-alert-icon" aria-hidden="true" focusable="false">%s</svg>',
    alert_icon_paths[kind]
  )
end

local function prepend_title_icon(title, kind)
  local icon = pandoc.RawInline("html", alert_icon(kind))
  if #title.content == 0 then
    title.content = { pandoc.Plain({ icon }) }
    return
  end

  local first = title.content[1]
  if first.t == "Para" or first.t == "Plain" then
    table.insert(first.content, 1, icon)
    return
  end

  table.insert(title.content, 1, pandoc.Plain({ icon }))
end

local function mermaid_attributes(attr)
  local identifier = ""
  local attributes = {}

  if attr and attr.identifier and attr.identifier ~= "" then
    identifier = attr.identifier
  end

  if attr and attr.attributes then
    for name, value in pairs(attr.attributes) do
      if safe_attribute_name(name) then
        attributes[name] = value
      end
    end
  end

  return identifier, attributes
end

function CodeBlock(el)
  local is_mermaid = false
  for _, class_name in ipairs(el.classes) do
    if class_name == "mermaid" then
      is_mermaid = true
      break
    end
  end
  if not is_mermaid then
    return nil
  end

  local identifier, attributes = mermaid_attributes(el.attr)
  local classes = { "mermaid" }
  for _, class_name in ipairs(el.classes) do
    if class_name ~= "mermaid" then
      table.insert(classes, class_name)
    end
  end

  return pandoc.Div(
    { pandoc.Plain({ pandoc.Str(el.text) }) },
    pandoc.Attr(identifier, classes, attributes)
  )
end

function RawInline(el)
  if el.format == "html" then
    return pandoc.Str(el.text)
  end
end

function RawBlock(el)
  if el.format == "html" then
    return pandoc.Para({ pandoc.Str(el.text) })
  end
end

function Div(el)
  local kind = alert_kind(el.classes)
  if not kind then
    return nil
  end
  local title = el.content[1]
  if not title or title.t ~= "Div" or not has_class(title, "title") then
    return nil
  end

  prepend_title_icon(title, kind)
  title.classes = { "markdown-alert-title" }
  el.classes = { "markdown-alert", "markdown-alert-" .. kind }
  return el
end

function Table(el)
  local attributes = {}
  local position = attr_value(el.attr, "data-pos")
  if position then
    attributes["data-pos"] = position
  end
  return pandoc.Div({ el }, pandoc.Attr("", { "table-scroll" }, attributes))
end

function Pandoc(doc)
  local source_path = PANDOC_STATE.input_files and PANDOC_STATE.input_files[1]
  if not source_path or source_path == "" then
    return nil
  end

  local source_text = pandoc.Span({
    pandoc.Span({ pandoc.Str("Source:") }, pandoc.Attr("", { "document-source__label" })),
    pandoc.Code(source_path, pandoc.Attr("", { "document-source__path" }, { title = source_path })),
  }, pandoc.Attr("", { "document-source__text" }))

  local light_theme_button = pandoc.RawInline("html", [[
    <button class="source-theme-button" type="button" data-theme="light" aria-label="Light theme" title="Light theme" aria-pressed="false">
      <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-sun preview-icon" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/></svg>
    </button>
  ]])

  local dark_theme_button = pandoc.RawInline("html", [[
    <button class="source-theme-button" type="button" data-theme="dark" aria-label="Dark theme" title="Dark theme" aria-pressed="false">
      <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-moon preview-icon" aria-hidden="true" focusable="false"><path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401"/></svg>
    </button>
  ]])

  local theme_switcher = pandoc.Span(
    { light_theme_button, dark_theme_button },
    pandoc.Attr("", { "md-view-theme-switcher", "not-prose" }, {
      role = "group",
      ["aria-label"] = "Reading theme",
    })
  )

  local copy_button = pandoc.RawInline("html", [[
    <button class="source-copy-button" type="button" aria-label="Copy source path">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" aria-hidden="true" focusable="false">
        <rect x="88" y="88" width="128" height="128" rx="8" fill="none" stroke="currentColor" stroke-width="16" stroke-linecap="round" stroke-linejoin="round"></rect>
        <path d="M168,88V48a8,8,0,0,0-8-8H48a8,8,0,0,0-8,8V160a8,8,0,0,0,8,8H88" fill="none" stroke="currentColor" stroke-width="16" stroke-linecap="round" stroke-linejoin="round"></path>
      </svg>
    </button>
  ]])

  local copy_status = pandoc.RawInline("html", [[
    <output class="source-copy-status" role="status" aria-live="polite" aria-atomic="true" hidden></output>
  ]])

  local banner = pandoc.Div(
    { pandoc.Plain({ source_text, copy_button, copy_status }) },
    pandoc.Attr("", { "document-source", "not-prose" }, { ["data-source-path"] = source_path })
  )

  local content = pandoc.Div(doc.blocks, pandoc.Attr("", { "document-content" }))
  local header = pandoc.MetaBlocks({
    pandoc.RawBlock("html", '<header class="md-view-header not-prose">'),
    pandoc.Plain({ theme_switcher }),
    pandoc.RawBlock("html", "</header>"),
  })
  local include_before = pandoc.MetaList({ header, banner })
  local existing_include_before = doc.meta["include-before"]
  if existing_include_before then
    if pandoc.utils.type(existing_include_before) == "MetaList" then
      for _, item in ipairs(existing_include_before) do
        include_before:insert(item)
      end
    else
      include_before:insert(existing_include_before)
    end
  end
  doc.meta["include-before"] = include_before
  doc.blocks = pandoc.Blocks({ content })
  return doc
end
