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
