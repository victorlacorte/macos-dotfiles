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
