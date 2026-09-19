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
		`.table-scroll`,
		"--tw-prose-body",
		"data:image/svg+xml",
		`data-pos="`,
		"explicit-heading",
		`id="repeated-heading"`,
		"<table",
		`<ul data-pos="`,
		`<blockquote data-pos="`,
		"flowchart TD",
		"sequenceDiagram",
		"Mermaid could not render this diagram",
		"node.textContent = source",
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
	} {
		if strings.Contains(renderedHTML, needle) {
			t.Errorf("rendered HTML unexpectedly contains %q", needle)
		}
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
	} {
		if !regexp.MustCompile(test.regex).MatchString(renderedHTML) {
			t.Errorf("%s: no match for %s", test.name, test.regex)
		}
	}

	bannerRe := regexp.MustCompile(
		`(?s)<div class="[^"]*\bdocument-source\b[^"]*\bnot-prose\b[^"]*" data-source-path="([^"]*)"[^>]*>(.*?)</div>`,
	)
	bannerMatch := bannerRe.FindStringSubmatch(renderedHTML)
	if bannerMatch == nil {
		t.Fatal("document source banner not found")
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
	contentMatch := regexp.MustCompile(`(?s)<span(?:\s[^>]*)?>\s*Source:\s*<code[^>]*>([^<]*)</code>\s*</span>`).
		FindStringSubmatch(bannerMatch[2])
	if contentMatch == nil {
		t.Fatal("source label and code-formatted path not found in banner span")
	}
	codePathEscaped := contentMatch[1]
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

	buttonMatch := regexp.MustCompile(`(?s)<button\b([^>]*)>(.*?)</button>`).FindStringSubmatch(bannerMatch[2])
	if buttonMatch == nil {
		t.Fatal("native source-copy button not found in banner")
	}
	for _, attribute := range []string{`type="button"`, `aria-label="Copy source path"`} {
		if !strings.Contains(buttonMatch[1], attribute) {
			t.Errorf("source-copy button missing %s", attribute)
		}
	}
	svgTag := regexp.MustCompile(`<svg\b([^>]*)>`).FindStringSubmatch(buttonMatch[2])
	if svgTag == nil {
		t.Fatal("source-copy button should contain its SVG icon")
	}
	for _, attribute := range []string{`aria-hidden="true"`, `viewBox="0 0 256 256"`} {
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
