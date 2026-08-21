package mdview_test

import (
	"bytes"
	"context"
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
	sourceDir := filepath.Join(root, "source with spaces and Unicode é")
	outputDir := filepath.Join(root, "output with spaces")
	if err := os.Mkdir(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	input := filepath.Join(sourceDir, "representative.md")
	copyFile(t, filepath.Join(dataDir, "test", "fixtures", "representative.md"), input)
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

	html := string(readFile(t, output))
	for _, needle := range []string{
		"<!DOCTYPE html>",
		"<style>",
		".table-scroll",
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
		if !strings.Contains(html, needle) {
			t.Errorf("rendered HTML missing %q", needle)
		}
	}
	for _, needle := range []string{
		"md-view.css",
		"local-image.svg",
		`<span class="unsafe">raw HTML`,
	} {
		if strings.Contains(html, needle) {
			t.Errorf("rendered HTML unexpectedly contains %q", needle)
		}
	}

	if got := len(regexp.MustCompile(`<script[^>]*src=`).FindAllString(html, -1)); got != 1 {
		t.Errorf("external script count = %d, want 1", got)
	}
	mermaidURL := "https://cdn.jsdelivr.net/npm/mermaid@11.12.1/dist/mermaid.min.js"
	if got := len(regexp.MustCompile(regexp.QuoteMeta(mermaidURL)).FindAllString(html, -1)); got != 1 {
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
		if !regexp.MustCompile(test.regex).MatchString(html) {
			t.Errorf("%s: no match for %s", test.name, test.regex)
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
