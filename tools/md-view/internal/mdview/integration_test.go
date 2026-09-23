package mdview_test

import (
	"bytes"
	"context"
	htmllib "html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/victorlacorte/macos-dotfiles/tools/md-view/internal/mdview"
)

func TestRenderRepresentativeFixture(t *testing.T) {
	pandocPath := resolvePandoc(t)
	dataDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	sourceDir := filepath.Join(root, "source with spaces & Unicode é")
	outputDir := filepath.Join(root, "output with spaces")
	if err := os.Mkdir(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	input := filepath.Join(sourceDir, "representative.md")
	copyFile(t, filepath.Join(dataDir, "test", "fixtures", "representative.md"), input)
	canonicalInput, err := filepath.Abs(input)
	if err != nil {
		t.Fatal(err)
	}
	canonicalInput, err = filepath.EvalSymlinks(canonicalInput)
	if err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(dataDir, "test", "fixtures", "local-image.svg"), filepath.Join(sourceDir, "local-image.svg"))
	output := filepath.Join(outputDir, "representative preview.html")

	var stdout, stderr bytes.Buffer
	app := &mdview.App{
		Runner: mdview.OSRunner{},
		LookPath: func(string) (string, error) {
			return pandocPath, nil
		},
		LookupEnv: func(name string) (string, bool) {
			if name == "MD_VIEW_DATA_DIR" {
				return dataDir, true
			}
			return "", false
		},
		Program: "md-view",
		Stdout:  &stdout,
		Stderr:  &stderr,
	}
	status := app.Main(context.Background(), []string{"render", input, "--output", output})
	if status != 0 {
		t.Fatalf("render status = %d, stderr=%q", status, stderr.String())
	}

	resolvedOutput, err := filepath.EvalSymlinks(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != resolvedOutput {
		t.Fatalf("stdout = %q, want %q", got, resolvedOutput)
	}

	renderedHTML := string(readFile(t, output))
	for _, needle := range []string{
		"<!DOCTYPE html>",
		"<style>",
		"document-source__path",
		`.table-scroll`,
		"68ch",
		"--tw-prose-body",
		"data:image/svg+xml",
		`data-pos="`,
		"explicit-heading",
		"fixture-callout",
		`class="markdown-alert markdown-alert-note"`,
		`class="markdown-alert markdown-alert-tip"`,
		`class="markdown-alert markdown-alert-important"`,
		`class="markdown-alert markdown-alert-warning"`,
		`class="markdown-alert markdown-alert-caution"`,
		`class="markdown-alert-title"`,
		`class="markdown-alert-icon"`,
		"https://example.com/note",
		`id="repeated-heading"`,
		"<table",
		`<ul data-pos="`,
		`<blockquote data-pos="`,
		"flowchart TD",
		"sequenceDiagram",
		"Author-&gt;&gt;Viewer: Save Markdown",
		"Viewer--&gt;&gt;Author: Render HTML",
		"Mermaid could not render this diagram",
		"node.textContent = source",
		"Mermaid diagram navigation",
		"MIN_ZOOM = 0.25",
		"MAX_ZOOM = 4",
		`var endAligned = initialStart + initialSize - viewSize`,
		`Math.min(initialStart, endAligned)`,
		`Math.max(initialStart, endAligned)`,
		`zoomOut.disabled = state.zoom <= MIN_ZOOM`,
		`(state.zoom < 1 && nextZoom > 1)`,
		`(state.zoom > 1 && nextZoom < 1)`,
		`state.zoom = clamp(nextZoom, MIN_ZOOM, MAX_ZOOM)`,
		`button.setAttribute("aria-label", label + " Mermaid diagram")`,
		`button.addEventListener("pointerdown"`,
		`button.addEventListener("keydown"`,
		`if (activatedByPointer)`,
		`button.blur()`,
		`svg.focus()`,
		`viewport.className = "mermaid-viewport"`,
		`viewport.appendChild(svg)`,
		`panUp.disabled = !canPan`,
		`panDown.disabled = !canPan`,
		`panLeft.disabled = !canPan`,
		`panRight.disabled = !canPan`,
		`event.key === "ArrowLeft"`,
		`window.addEventListener("beforeprint"`,
		`window.addEventListener("afterprint"`,
		`svg.setAttribute("tabindex", "0")`,
		`toolbar.setAttribute("role", "toolbar")`,
		`dialog.showModal()`,
		`prefixSvgIds(`,
		`return "url(#" + next + ")"`,
		`event.target === dialog`,
		`var generation = renderGeneration + 1`,
		`navigationControllers = []`,
		`generation === renderGeneration`,
		"div.sourceCode",
		"&lt;span",
	} {
		if !strings.Contains(renderedHTML, needle) {
			t.Errorf("rendered HTML missing %q", needle)
		}
	}
	for _, needle := range []string{
		"md-view.css",
		"local-image.svg",
		`<span class="unsafe">raw HTML`,
		`role="alert"`,
		`<div class="title"`,
	} {
		if strings.Contains(renderedHTML, needle) {
			t.Errorf("rendered HTML unexpectedly contains %q", needle)
		}
	}
	var embeddedCSS string
	for _, style := range regexp.MustCompile(`(?s)<style\b[^>]*>(.*?)</style>`).FindAllStringSubmatch(renderedHTML, -1) {
		if strings.Contains(style[1], ".document-source__path") && strings.Contains(style[1], "68ch") {
			embeddedCSS = style[1]
			break
		}
	}
	if embeddedCSS == "" {
		t.Fatal("embedded stylesheet not found")
	}
	for _, fragment := range []string{
		"max-width: calc(68ch + var(--md-view-page-gutter) + var(--md-view-page-gutter));",
		"padding: 3rem var(--md-view-page-gutter);",
		"--md-view-page-gutter",
		"text-overflow: ellipsis;",
		"white-space: nowrap;",
		"overflow-wrap: anywhere;",
		"--md-view-scrim: rgb(0 0 0 / 45%);",
		"--md-view-scrim: rgb(255 255 255 / 16%);",
	} {
		if !strings.Contains(embeddedCSS, fragment) {
			t.Errorf("embedded CSS missing %q", fragment)
		}
	}
	for _, rule := range []string{
		`(?s)\.document-source__text\s*\{[^}]*min-width:\s*0;`,
		`(?s)\.document-source__path\s*\{[^}]*min-width:\s*0;[^}]*overflow:\s*hidden;[^}]*text-overflow:\s*ellipsis;[^}]*white-space:\s*nowrap;`,
		`(?s)\.md-view-header\s*\{[^}]*display:\s*flex;`,
		`(?s)\.md-view-header\s*\{[^}]*justify-content:\s*flex-end;`,
		`(?s)\.document-content pre\s*\{[^}]*overflow-x:\s*auto;`,
		`(?s)\.md-view-theme-switcher\s*\{[^}]*display:\s*inline-flex;`,
		`(?s)\.table-scroll\s*\{[^}]*overflow-x:\s*auto;`,
		`(?s)\.mermaid\s*\{[^}]*position:\s*relative;[^}]*overflow:\s*hidden;`,
		`(?s)\.mermaid-viewport\s*\{[^}]*min-height:\s*9\.5rem;[^}]*overflow:\s*auto;`,
		`(?s)\.mermaid-toolbar\s*\{[^}]*grid-template-areas:`,
		`(?s)\.mermaid-toolbar button\s*\{[^}]*color:\s*var\(--md-view-high-emphasis\);[^}]*background:\s*var\(--md-view-page-background\);`,
		`(?s)@media print\s*\{.*?\.document-source__path\s*\{[^}]*white-space:\s*normal;`,
		`(?s)@media print\s*\{[^}]*\.md-view-header[^}]*display:\s*none\s*!important;`,
		`(?s)\.callout\s*\{[^}]*border-left:\s*0\.25rem solid var\(--md-view-link\);`,
		`(?s)\.markdown-alert\s*\{[^}]*border-left:\s*0\.25rem solid var\(--md-view-boundary\);`,
		`(?s)\.markdown-alert-icon\s*\{[^}]*width:\s*1rem;[^}]*height:\s*1rem;`,
		`(?s)\.markdown-alert-note\s*\{[^}]*border-left-color:\s*var\(--md-view-link\);`,
		`(?s)\.markdown-alert-tip\s*\{[^}]*border-left-color:\s*var\(--md-view-success-text\);`,
		`(?s)\.markdown-alert-important\s*\{[^}]*border-left-color:\s*var\(--md-view-important-text\);`,
		`(?s)\.markdown-alert-warning\s*\{[^}]*border-left-color:\s*var\(--md-view-warning-text\);`,
		`(?s)\.markdown-alert-caution\s*\{[^}]*border-left-color:\s*var\(--md-view-error-text\);`,
		`(?s)@media print\s*\{.*?\.markdown-alert-note\s*\{[^}]*border-left-color:\s*#0969da;`,
		`(?s)@media print\s*\{[^}]*\.md-view-theme-switcher[^}]*display:\s*none\s*!important;`,
		`(?s)@media print\s*\{.*?\.mermaid-toolbar\s*\{[^}]*display:\s*none;`,
		`(?s)\.mermaid-dialog\s*\{[^}]*width:\s*calc\(100vw - 2rem\);[^}]*height:\s*calc\(100vh - 2rem\);`,
		`(?s)\.mermaid-dialog::backdrop\s*\{[^}]*background:\s*var\(--md-view-scrim\);`,
		`(?s)\.mermaid-expand\s*\{[^}]*grid-area:\s*expand;`,
		`(?s)@media print\s*\{.*?\.mermaid-dialog\s*\{[^}]*display:\s*none;`,
	} {
		if !regexp.MustCompile(rule).MatchString(embeddedCSS) {
			t.Errorf("embedded CSS missing rule %q", rule)
		}
	}
	if regexp.MustCompile(`(?s)body\s*\{[^}]*grid-template-columns`).MatchString(embeddedCSS) ||
		strings.Contains(embeddedCSS, "grid-column:") {
		t.Error("embedded CSS should not contain the three-column reading grid or wide grid escapes")
	}
	if strings.Contains(embeddedCSS, "76rem") || strings.Contains(embeddedCSS, "position: sticky") {
		t.Error("embedded CSS should not contain a wide canvas or sticky header")
	}
	if !strings.Contains(embeddedCSS, "@media print") || !strings.Contains(embeddedCSS, ".source-copy-button") {
		t.Error("embedded CSS should retain print rules that hide interactive controls")
	}
	if got := len(regexp.MustCompile(`<div class="table-scroll"`).FindAllString(renderedHTML, -1)); got != 2 {
		t.Errorf("table wrapper count = %d, want narrow and wide table wrappers", got)
	}
	renderedText := htmllib.UnescapeString(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(renderedHTML, ""))
	renderedText = strings.Join(strings.Fields(renderedText), " ")
	for _, fragment := range []string{
		"alpha",
		"A very wide value that deliberately exercises horizontal scrolling",
		"ordinary fenced code remains code",
		"flowchart TD",
		"sequenceDiagram",
		"nested alert item",
		"Unknown markers stay ordinary block quotes.",
		"This fenced note div is not an alert.",
		"A note with emphasis",
	} {
		if !strings.Contains(renderedText, fragment) {
			t.Errorf("representative fixture content missing %q", fragment)
		}
	}
	if !strings.Contains(renderedHTML, `alt="Local fixture image"`) {
		t.Error("embedded local image is missing its alternative text")
	}

	for _, label := range []string{
		"Expand",
		"Pan up",
		"Zoom in",
		"Pan left",
		"Reset",
		"Pan right",
		"Pan down",
		"Zoom out",
	} {
		needle := `createButton("` + label + `",`
		if !strings.Contains(renderedHTML, needle) {
			t.Errorf("rendered HTML missing control %q", label)
		}
	}
	if got := strings.Count(renderedHTML, `createButton("`); got != 8 {
		t.Errorf("control count = %d, want 8", got)
	}

	if got := len(regexp.MustCompile(`<script[^>]*src=`).FindAllString(renderedHTML, -1)); got != 1 {
		t.Errorf("external script count = %d, want 1", got)
	}
	mermaidURL := "https://cdn.jsdelivr.net/npm/mermaid@11.12.1/dist/mermaid.min.js"
	if got := len(regexp.MustCompile(regexp.QuoteMeta(mermaidURL)).FindAllString(renderedHTML, -1)); got != 1 {
		t.Errorf("Mermaid URL count = %d, want 1", got)
	}

	for _, test := range []struct {
		name  string
		regex string
	}{
		{name: "mermaid data-pos", regex: `<div id="[^"]*" class="mermaid" data-pos="`},
		{name: "table-scroll data-pos", regex: `<div class="table-scroll" data-pos="`},
		{name: "explicit heading data-pos", regex: `<h2 data-pos="[^"]*" id="explicit-heading"`},
		{name: "repeated heading data-pos", regex: `<h2 data-pos="[^"]*" id="repeated-heading"`},
		{name: "note alert data-pos", regex: `<div class="markdown-alert markdown-alert-note" data-pos="`},
		{name: "tip alert data-pos", regex: `<div class="markdown-alert markdown-alert-tip" data-pos="`},
		{name: "important alert data-pos", regex: `<div class="markdown-alert markdown-alert-important" data-pos="`},
		{name: "warning alert data-pos", regex: `<div class="markdown-alert markdown-alert-warning" data-pos="`},
		{name: "caution alert data-pos", regex: `<div class="markdown-alert markdown-alert-caution" data-pos="`},
	} {
		if !regexp.MustCompile(test.regex).MatchString(renderedHTML) {
			t.Errorf("%s: no match for %s", test.name, test.regex)
		}
	}

	if got := strings.Count(renderedHTML, `class="markdown-alert `); got != 5 {
		t.Errorf("namespaced alert count = %d, want 5", got)
	}
	if got := strings.Count(renderedHTML, `id="fixture-callout"`); got != 1 {
		t.Errorf("callout count = %d, want 1", got)
	}
	if !regexp.MustCompile(`<div class="note"`).MatchString(renderedHTML) {
		t.Error("fenced note div should remain class=\"note\"")
	}
	if regexp.MustCompile(`<div class="note"[^>]*markdown-alert`).MatchString(renderedHTML) {
		t.Error("fenced note div should not receive markdown-alert classes")
	}
	var unknownQuote string
	for _, quote := range regexp.MustCompile(`(?s)<blockquote data-pos="[^"]*">.*?</blockquote>`).FindAllString(renderedHTML, -1) {
		if strings.Contains(quote, "UNKNOWN") {
			unknownQuote = quote
			break
		}
	}
	if unknownQuote == "" {
		t.Error("unknown alert marker should remain an ordinary blockquote")
	}
	if strings.Contains(unknownQuote, "markdown-alert") {
		t.Error("unknown alert marker should not be rewritten as a namespaced alert")
	}

	alertTitles := map[string]string{
		"note":      "Note",
		"tip":       "Tip",
		"important": "Important",
		"warning":   "Warning",
		"caution":   "Caution",
	}
	iconPaths := map[string]string{
		"note":      "M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5",
		"tip":       "M8 1.5c-2.363 0-4 1.69-4 3.75",
		"important": "M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5",
		"warning":   "M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378",
		"caution":   "M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25",
	}
	for _, kind := range []string{"note", "tip", "important", "warning", "caution"} {
		start := strings.Index(renderedHTML, `<div class="markdown-alert markdown-alert-`+kind+`"`)
		if start < 0 {
			t.Errorf("%s alert HTML not found", kind)
			continue
		}
		next := strings.Index(renderedHTML[start+1:], `<div class="markdown-alert markdown-alert-`)
		end := len(renderedHTML)
		if next >= 0 {
			end = start + 1 + next
		}
		content := renderedHTML[start:end]
		titleMatch := regexp.MustCompile(`(?s)<div class="markdown-alert-title">(.*?)</div>`).FindStringSubmatch(content)
		if titleMatch == nil {
			t.Errorf("%s alert missing markdown-alert-title", kind)
			continue
		}
		if !strings.Contains(titleMatch[1], "</svg>"+alertTitles[kind]+"</p>") {
			t.Errorf("%s alert missing canonical title %q", kind, alertTitles[kind])
		}
		svgTag := regexp.MustCompile(`<svg\b([^>]*)>`).FindStringSubmatch(titleMatch[1])
		if svgTag == nil {
			t.Errorf("%s alert title missing decorative SVG", kind)
			continue
		}
		for _, attribute := range []string{
			`class="markdown-alert-icon"`,
			`aria-hidden="true"`,
			`focusable="false"`,
			`viewBox="0 0 16 16"`,
		} {
			if !strings.Contains(svgTag[1], attribute) {
				t.Errorf("%s alert icon missing %s", kind, attribute)
			}
		}
		if !strings.Contains(titleMatch[1], iconPaths[kind]) {
			t.Errorf("%s alert icon path mismatch", kind)
		}
	}
	noteStart := strings.Index(renderedHTML, `<div class="markdown-alert markdown-alert-note"`)
	noteEnd := strings.Index(renderedHTML, `<div class="markdown-alert markdown-alert-tip"`)
	if noteStart < 0 || noteEnd < 0 || noteEnd <= noteStart {
		t.Fatal("note alert body not found")
	}
	noteAlert := renderedHTML[noteStart:noteEnd]
	for _, fragment := range []string{"<strong>", `href="https://example.com/note"`, "<code ", "<li>"} {
		if !strings.Contains(noteAlert, fragment) {
			t.Errorf("note alert missing nested Markdown HTML %q", fragment)
		}
	}

	bannerRe := regexp.MustCompile(
		`(?s)<div class="[^"]*\bdocument-source\b[^"]*\bnot-prose\b[^"]*" data-source-path="([^"]*)"[^>]*>(.*?)</div>`,
	)
	bannerMatch := bannerRe.FindStringSubmatch(renderedHTML)
	if bannerMatch == nil {
		t.Fatal("document source banner not found")
	}
	mdViewHeaderRe := regexp.MustCompile(`(?s)<header class="md-view-header not-prose">(.*?)</header>`)
	mdViewHeaderMatch := mdViewHeaderRe.FindStringSubmatch(renderedHTML)
	if mdViewHeaderMatch == nil {
		t.Fatal("semantic md-view header not found")
	}
	mdViewHeaderBounds := mdViewHeaderRe.FindStringIndex(renderedHTML)
	bannerBounds := bannerRe.FindStringIndex(renderedHTML)
	if mdViewHeaderBounds[1] >= bannerBounds[0] {
		t.Error("md-view header should be a separate section before the document source banner")
	}
	if strings.Contains(mdViewHeaderMatch[1], "document-source") {
		t.Error("md-view header should not contain the document source banner")
	}
	if strings.Contains(bannerMatch[2], "md-view-theme-switcher") {
		t.Error("document source banner should not contain the theme switcher")
	}
	if strings.Contains(bannerMatch[2], "document-source__actions") {
		t.Error("document source banner should not have a separate actions group")
	}
	titleHeaderBounds := regexp.MustCompile(`<header\b[^>]*id="title-block-header"[^>]*>`).FindStringIndex(renderedHTML)
	if titleHeaderBounds == nil {
		t.Fatal("Pandoc YAML title block not found")
	}
	documentContentBounds := regexp.MustCompile(`<([[:alpha:]]+)\b[^>]*class="[^"]*\bdocument-content\b[^"]*"[^>]*>`).FindStringIndex(renderedHTML)
	if documentContentBounds == nil {
		t.Fatal("document content wrapper not found")
	}
	documentContentStart := documentContentBounds[0]
	if mdViewHeaderBounds[1] >= titleHeaderBounds[0] {
		t.Error("md-view header should appear before Pandoc's YAML title block")
	}
	if mdViewHeaderBounds[1] >= documentContentStart {
		t.Error("md-view header should appear before the document content wrapper")
	}
	firstIncludeBeforeIndex := strings.Index(renderedHTML, "Existing include-before.")
	secondIncludeBeforeIndex := strings.Index(renderedHTML, "Second include-before.")
	if firstIncludeBeforeIndex < 0 || secondIncludeBeforeIndex < 0 {
		t.Fatal("pre-existing include-before metadata was not preserved")
	}
	if firstIncludeBeforeIndex < bannerBounds[1] ||
		secondIncludeBeforeIndex < firstIncludeBeforeIndex || secondIncludeBeforeIndex >= titleHeaderBounds[0] {
		t.Error("pre-existing include-before content should follow the app header and source banner, then precede the YAML title block")
	}
	bodyEnd := strings.LastIndex(renderedHTML, "</body>")
	if bodyEnd < documentContentStart {
		t.Fatal("document content wrapper is outside the HTML body")
	}
	documentContent := renderedHTML[documentContentStart:bodyEnd]
	for _, needle := range []string{`id="explicit-heading"`, `<ul data-pos="`, `<blockquote data-pos="`} {
		if !strings.Contains(documentContent, needle) {
			t.Errorf("document content wrapper missing source-positioned child %q", needle)
		}
	}
	for _, fragment := range []string{" ", "&", "é"} {
		if !strings.Contains(canonicalInput, fragment) {
			t.Fatalf("canonical input %q does not exercise path fragment %q", canonicalInput, fragment)
		}
	}

	escapedCanonicalInput := htmllib.EscapeString(canonicalInput)
	if bannerMatch[1] != escapedCanonicalInput {
		t.Errorf("raw data-source-path = %q, want escaped path %q", bannerMatch[1], escapedCanonicalInput)
	}
	if !strings.Contains(bannerMatch[2], `class="document-source__text"`) {
		t.Fatal("source label and path container not found in banner")
	}
	if !strings.Contains(bannerMatch[2], `class="document-source__label"`) || !strings.Contains(bannerMatch[2], "Source:") {
		t.Error("source label hook or visible label missing from banner")
	}
	pathMatch := regexp.MustCompile(`(?s)<code\b([^>]*)>([^<]*)</code>`).FindStringSubmatch(bannerMatch[2])
	if pathMatch == nil {
		t.Fatal("code-formatted source path not found in banner")
	}
	for _, attribute := range []string{`class="document-source__path"`, `title="` + escapedCanonicalInput + `"`} {
		if !strings.Contains(pathMatch[1], attribute) {
			t.Errorf("visible source path missing %s", attribute)
		}
	}
	codePathEscaped := pathMatch[2]
	if codePathEscaped != escapedCanonicalInput {
		t.Errorf("raw banner code path = %q, want escaped path %q", codePathEscaped, escapedCanonicalInput)
	}
	attrPath := htmllib.UnescapeString(bannerMatch[1])
	codePath := htmllib.UnescapeString(codePathEscaped)
	if attrPath != canonicalInput {
		t.Errorf("data-source-path = %q, want %q", attrPath, canonicalInput)
	}
	if codePath != canonicalInput {
		t.Errorf("banner code path = %q, want %q", codePath, canonicalInput)
	}

	themeGroupMatch := regexp.MustCompile(`(?s)<span\b([^>]*\bmd-view-theme-switcher\b[^>]*)>(.*?)</span>`).
		FindStringSubmatch(mdViewHeaderMatch[1])
	if themeGroupMatch == nil {
		t.Fatal("theme switcher group not found in the md-view header")
	}
	themeGroupAttributes := themeGroupMatch[1]
	themeGroupBody := themeGroupMatch[2]
	for _, attribute := range []string{`role="group"`, `aria-label="Reading theme"`} {
		if !strings.Contains(themeGroupAttributes, attribute) {
			t.Errorf("theme switcher group missing %s", attribute)
		}
	}
	themeButtons := regexp.MustCompile(`(?s)<button\b([^>]*)>(.*?)</button>`).FindAllStringSubmatch(themeGroupBody, -1)
	if len(themeButtons) != 2 {
		t.Fatalf("theme button count = %d, want 2", len(themeButtons))
	}
	expectedThemeButtons := map[string]string{"light": "Light theme", "dark": "Dark theme"}
	seenThemes := make(map[string]bool)
	for _, button := range themeButtons {
		themeMatch := regexp.MustCompile(`\bdata-theme="([^"]+)"`).FindStringSubmatch(button[1])
		if themeMatch == nil {
			t.Errorf("theme button missing data-theme attribute: %s", button[1])
			continue
		}
		theme := themeMatch[1]
		seenThemes[theme] = true
		label, ok := expectedThemeButtons[theme]
		if !ok {
			t.Errorf("unsupported theme button value %q", theme)
			continue
		}
		for _, attribute := range []string{
			`type="button"`,
			`aria-label="` + label + `"`,
			`title="` + label + `"`,
			`aria-pressed="false"`,
		} {
			if !strings.Contains(button[1], attribute) {
				t.Errorf("%s theme button missing %s", theme, attribute)
			}
		}
		svgTag := regexp.MustCompile(`<svg\b([^>]*)>`).FindStringSubmatch(button[2])
		if svgTag == nil {
			t.Errorf("%s theme button should contain its inline SVG icon", theme)
			continue
		}
		for _, attribute := range []string{`aria-hidden="true"`, `focusable="false"`, `viewBox="0 0 24 24"`} {
			if !strings.Contains(svgTag[1], attribute) {
				t.Errorf("%s theme SVG missing %s", theme, attribute)
			}
		}
		if theme == "light" && !strings.Contains(button[2], `<circle`) {
			t.Error("light theme button should contain the sun SVG circle")
		}
		if theme == "dark" {
			if !strings.Contains(button[2], `M20.985 12.486`) {
				t.Error("dark theme button should contain the moon SVG path")
			}
			if strings.Contains(button[2], `<circle`) {
				t.Error("dark theme button should not contain a circle")
			}
		}
	}
	if !seenThemes["light"] || !seenThemes["dark"] || len(seenThemes) != 2 {
		t.Errorf("theme buttons = %v, want exactly light and dark", seenThemes)
	}

	copyButtonMatch := regexp.MustCompile(`(?s)<button\b([^>]*class="[^"]*\bsource-copy-button\b[^"]*"[^>]*)>(.*?)</button>`).
		FindStringSubmatch(bannerMatch[2])
	if copyButtonMatch == nil {
		t.Fatal("native source-copy button not found in banner")
	}
	for _, attribute := range []string{`type="button"`, `aria-label="Copy source path"`} {
		if !strings.Contains(copyButtonMatch[1], attribute) {
			t.Errorf("source-copy button missing %s", attribute)
		}
	}
	svgTag := regexp.MustCompile(`<svg\b([^>]*)>`).FindStringSubmatch(copyButtonMatch[2])
	if svgTag == nil {
		t.Fatal("source-copy button should contain its SVG icon")
	}
	for _, attribute := range []string{`aria-hidden="true"`, `focusable="false"`, `viewBox="0 0 256 256"`} {
		if !strings.Contains(svgTag[1], attribute) {
			t.Errorf("source-copy SVG missing %s", attribute)
		}
	}

	statusTag := regexp.MustCompile(`<output\b([^>]*)>`).FindStringSubmatch(bannerMatch[2])
	if statusTag == nil {
		t.Fatal("source-copy status output not found in banner")
	}
	for _, attribute := range []string{`role="status"`, `aria-live="polite"`, `aria-atomic="true"`} {
		if !strings.Contains(statusTag[1], attribute) {
			t.Errorf("source-copy status output missing %s", attribute)
		}
	}
	if !regexp.MustCompile(`(?:^|\s)hidden(?:\s|=|$)`).MatchString(statusTag[1]) {
		t.Error("source-copy status output should be hidden initially")
	}

	var sourceCopyScript string
	for _, script := range regexp.MustCompile(`(?s)<script\b([^>]*)>(.*?)</script>`).FindAllStringSubmatch(renderedHTML, -1) {
		if strings.Contains(script[2], "navigator.clipboard") && strings.Contains(script[2], "dataset.sourcePath") {
			if regexp.MustCompile(`(?i)\bsrc\s*=`).MatchString(script[1]) {
				t.Error("source-copy script should be inline")
			}
			sourceCopyScript = script[2]
			break
		}
	}
	if sourceCopyScript == "" {
		t.Fatal("inline source-copy script reading data-source-path not found")
	}
	for _, message := range []string{"Source copied to clipboard.", "Could not copy source."} {
		if !strings.Contains(sourceCopyScript, message) {
			t.Errorf("source-copy script missing exact feedback message %q", message)
		}
	}
	writeCall := regexp.MustCompile(`navigator\.clipboard\.writeText\s*\(\s*([A-Za-z_$][A-Za-z0-9_$]*(?:\.dataset\.sourcePath)?)\s*\)`).
		FindStringSubmatch(sourceCopyScript)
	if writeCall == nil {
		t.Fatal("source-copy script should call navigator.clipboard.writeText with the source path")
	}
	clipboardInput := writeCall[1]
	if !strings.HasSuffix(clipboardInput, ".dataset.sourcePath") {
		assignment := regexp.MustCompile(`(?:const|let|var)\s+` + regexp.QuoteMeta(clipboardInput) + `\s*=\s*[A-Za-z_$][A-Za-z0-9_$]*\.dataset\.sourcePath\b`)
		if !assignment.MatchString(sourceCopyScript) {
			t.Errorf("navigator.clipboard.writeText input %q is not read from banner.dataset.sourcePath", clipboardInput)
		}
	}

	var themeScript string
	for _, script := range regexp.MustCompile(`(?s)<script\b([^>]*)>(.*?)</script>`).FindAllStringSubmatch(renderedHTML, -1) {
		if strings.Contains(script[2], `supportedThemes = ["light", "dark"]`) && strings.Contains(script[2], `data-md-view-theme`) {
			if regexp.MustCompile(`(?i)\bsrc\s*=`).MatchString(script[1]) {
				t.Error("theme script should be inline")
			}
			themeScript = script[2]
			break
		}
	}
	if themeScript == "" {
		t.Fatal("inline theme initialization script not found")
	}
	if themeScriptIndex := strings.Index(renderedHTML, themeScript); themeScriptIndex < 0 || themeScriptIndex > strings.Index(renderedHTML, "<body") {
		t.Error("theme initialization script should run from the document head before body rendering")
	}
	for _, fragment := range []string{
		`const defaultTheme =`,
		`window.matchMedia("(prefers-color-scheme: dark)").matches`,
		`const supportedThemes = ["light", "dark"];`,
		`.md-view-theme-switcher [data-theme]`,
		`return supportedThemes.indexOf(theme) !== -1;`,
		`root.setAttribute("data-md-view-theme", theme);`,
		`button.setAttribute("aria-pressed", buttonTheme === theme ? "true" : "false");`,
		`button.addEventListener("click"`,
		`setTheme(defaultTheme);`,
	} {
		if !strings.Contains(themeScript, fragment) {
			t.Errorf("theme script missing %q", fragment)
		}
	}
	if regexp.MustCompile(`(?i)\b(?:localStorage|sessionStorage|document\.cookie|fetch|XMLHttpRequest|WebSocket|EventSource)\b`).MatchString(themeScript) {
		t.Error("theme script should not access storage, cookies, or network APIs")
	}

	srcCSS := string(readFile(t, filepath.Join(dataDir, "pandoc/styles/md-view.src.css")))
	generatedCSS := string(readFile(t, filepath.Join(dataDir, "pandoc/styles/md-view.css")))
	themeIdentifier := regexp.MustCompile(`(?:data-md-view-theme|data-theme)="([^"]+)"`)
	for _, sample := range []struct {
		name    string
		content string
	}{
		{name: "rendered HTML", content: renderedHTML},
		{name: "source stylesheet", content: srcCSS},
		{name: "generated stylesheet", content: generatedCSS},
	} {
		for _, match := range themeIdentifier.FindAllStringSubmatch(sample.content, -1) {
			if match[1] != "light" && match[1] != "dark" {
				t.Errorf("%s has unsupported theme identifier %q", sample.name, match[1])
			}
		}
		for _, leftover := range []string{
			`data-theme="cool"`,
			`data-theme="warm"`,
			`data-md-view-theme="cool"`,
			`data-md-view-theme="warm"`,
			"--color-stone-",
		} {
			if strings.Contains(sample.content, leftover) {
				t.Errorf("%s unexpectedly contains %q", sample.name, leftover)
			}
		}
	}
	for _, token := range []string{
		"--color-gray-50",
		"--color-gray-100",
		"--color-gray-200",
		"--color-sky-700",
		"--color-green-50",
		"--color-red-50",
		"--color-violet-700",
		"--color-amber-800",
	} {
		if !strings.Contains(generatedCSS, token) {
			t.Errorf("generated stylesheet missing light token %q", token)
		}
	}

	temps, err := filepath.Glob(filepath.Join(outputDir, "*.tmp.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temps) != 0 {
		t.Errorf("temporary render files left behind: %v", temps)
	}

	artifacts, err := filepath.Glob(filepath.Join(dataDir, "test", "fixtures", "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 0 {
		t.Errorf("fixture directory received an output artifact: %v", artifacts)
	}
}

func TestRenderMermaidSourceStaysIntact(t *testing.T) {
	pandocPath := resolvePandoc(t)
	dataDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	fixturePath := filepath.Join(dataDir, "test", "fixtures", "mermaid-wrap.md")
	sourceMarkdown := string(readFile(t, fixturePath))
	source := mermaidFence(t, sourceMarkdown)

	root := t.TempDir()
	input := filepath.Join(root, "mermaid-wrap.md")
	copyFile(t, fixturePath, input)
	output := filepath.Join(root, "mermaid-wrap.html")

	var stdout, stderr bytes.Buffer
	app := &mdview.App{
		Runner: mdview.OSRunner{},
		LookPath: func(string) (string, error) {
			return pandocPath, nil
		},
		LookupEnv: func(name string) (string, bool) {
			if name == "MD_VIEW_DATA_DIR" {
				return dataDir, true
			}
			return "", false
		},
		Program: "md-view",
		Stdout:  &stdout,
		Stderr:  &stderr,
	}
	status := app.Main(context.Background(), []string{"render", input, "--output", output})
	if status != 0 {
		t.Fatalf("render status = %d, stderr=%q", status, stderr.String())
	}

	renderedHTML := string(readFile(t, output))
	divs := regexp.MustCompile(`(?s)<div\b[^>]*\bclass="mermaid"[^>]*>(.*?)</div>`).FindAllStringSubmatch(renderedHTML, -1)
	if len(divs) != 1 {
		t.Fatalf("mermaid div count = %d, want 1", len(divs))
	}
	inner := divs[0][1]
	if strings.Contains(inner, "<") {
		t.Fatalf("mermaid HTML contains a raw <, which could become a live tag: %q", inner)
	}

	longMessage := "Paywall-&gt;&gt;Offerings: GET offerings (productType=addon, productSku=protect)"
	if !strings.Contains(inner, longMessage) {
		t.Fatalf("long message is not one HTML line: %q", inner)
	}
	special := "compare a &lt; b &amp; b &gt; c"
	if !strings.Contains(inner, special) {
		t.Fatalf("special characters were not escaped as %q\nhtml: %q", special, inner)
	}

	got := htmllib.UnescapeString(inner)
	got = strings.TrimPrefix(got, "\n")
	if got != source {
		t.Fatalf("unescaped mermaid text = %q, want %q", got, source)
	}
}

func mermaidFence(t *testing.T, markdown string) string {
	t.Helper()
	const open = "```mermaid\n"
	start := strings.Index(markdown, open)
	if start < 0 {
		t.Fatal("fixture missing mermaid fence")
	}
	rest := markdown[start+len(open):]
	end := strings.Index(rest, "\n```")
	if end < 0 {
		t.Fatal("fixture mermaid fence is not closed")
	}
	return rest[:end+1]
}

func resolvePandoc(t *testing.T) string {
	t.Helper()
	if path, err := exec.LookPath("pandoc"); err == nil && executableFile(path) {
		return path
	}
	if _, err := exec.LookPath("mise"); err == nil {
		output, err := exec.Command("mise", "which", "pandoc").Output()
		if err == nil {
			path := strings.TrimSpace(string(output))
			if executableFile(path) {
				return path
			}
		}
	}
	t.Fatal("pandoc is required; run mise install pandoc first")
	return ""
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	contents, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, contents, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}
