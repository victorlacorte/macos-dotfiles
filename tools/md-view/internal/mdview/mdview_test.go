package mdview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      Options
		wantErr   string
		wantUsage bool
	}{
		{name: "no arguments", wantUsage: true},
		{name: "default view", args: []string{"README.md"}, want: Options{Mode: ViewMode, Input: "README.md"}},
		{name: "help", args: []string{"--help"}, want: Options{Help: true}},
		{name: "short help", args: []string{"-h"}, want: Options{Help: true}},
		{
			name: "render input before output",
			args: []string{"render", "README.md", "--output", "preview.html"},
			want: Options{Mode: RenderMode, Input: "README.md", Output: "preview.html"},
		},
		{
			name: "render output before input",
			args: []string{"render", "--output=preview.html", "README.md"},
			want: Options{Mode: RenderMode, Input: "README.md", Output: "preview.html"},
		},
		{
			name: "short output and terminator",
			args: []string{"render", "-o", "preview.html", "--", "README.md"},
			want: Options{Mode: RenderMode, Input: "README.md", Output: "preview.html"},
		},
		{
			name:      "terminator before input",
			args:      []string{"render", "--", "-README.md", "--output=preview.html"},
			wantErr:   "",
			wantUsage: true,
		},
		{name: "extra default argument", args: []string{"one.md", "two.md"}, wantUsage: true},
		{name: "help with extra argument", args: []string{"--help", "extra"}, wantUsage: true},
		{name: "render missing input", args: []string{"render", "--output", "preview.html"}, wantErr: "render requires an input Markdown file"},
		{name: "render missing output", args: []string{"render", "README.md"}, wantErr: "render requires --output OUTPUT.html"},
		{name: "unknown option", args: []string{"render", "README.md", "--wat"}, wantErr: "unknown option: --wat"},
		{name: "multiple inputs", args: []string{"render", "one.md", "two.md", "--output", "preview.html"}, wantErr: "multiple input files"},
		{name: "multiple outputs", args: []string{"render", "one.md", "-o", "one.html", "--output", "two.html"}, wantErr: "multiple output paths"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseArgs(test.args)
			if test.wantUsage || test.wantErr != "" {
				if err == nil {
					t.Fatalf("parseArgs(%q) succeeded, want error", test.args)
				}
				var usageErr *usageError
				if !errors.As(err, &usageErr) {
					t.Fatalf("parseArgs(%q) error = %T, want usageError", test.args, err)
				}
				if usageErr.message != test.wantErr {
					t.Fatalf("parseArgs(%q) message = %q, want %q", test.args, usageErr.message, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs(%q) error = %v", test.args, err)
			}
			if got != test.want {
				t.Fatalf("parseArgs(%q) = %#v, want %#v", test.args, got, test.want)
			}
		})
	}
}

func TestPosixCksumGoldenVectors(t *testing.T) {
	tests := []struct {
		input string
		want  uint32
	}{
		{input: "", want: 4294967295},
		{input: "a", want: 1220704766},
		{input: "123456789", want: 930766865},
		{input: "hello world", want: 1135714720},
		{input: "The quick brown fox jumps over the lazy dog", want: 2074844392},
	}

	for _, test := range tests {
		if got := posixCksum([]byte(test.input)); got != test.want {
			t.Errorf("posixCksum(%q) = %d, want %d", test.input, got, test.want)
		}
	}
}

func TestResolveDataDir(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "repository")
	if err := os.MkdirAll(filepath.Join(repository, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repository, "pandoc"), 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(repository, "bin", "md-view")
	writeFile(t, executable, "binary")

	noEnv := func(string) (string, bool) { return "", false }
	got, err := resolveDataDir(noEnv, executable, executable)
	if err != nil {
		t.Fatalf("repository-relative data lookup failed: %v", err)
	}
	want, _ := canonicalPath(repository)
	if got != want {
		t.Fatalf("repository-relative data dir = %q, want %q", got, want)
	}

	installed := filepath.Join(root, "installed")
	if err := os.MkdirAll(filepath.Join(installed, "bin", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(installed, "share", "md-view", "pandoc"), 0o755); err != nil {
		t.Fatal(err)
	}
	installedExecutable := filepath.Join(installed, "bin", "md-view")
	writeFile(t, installedExecutable, "binary")
	got, err = resolveDataDir(noEnv, installedExecutable, installedExecutable)
	if err != nil {
		t.Fatalf("installed data lookup failed: %v", err)
	}
	want, _ = canonicalPath(filepath.Join(installed, "share", "md-view"))
	if got != want {
		t.Fatalf("installed data dir = %q, want %q", got, want)
	}

	override := filepath.Join(root, "override")
	if err := os.MkdirAll(filepath.Join(override, "pandoc"), 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "override-alias")
	if err := os.Symlink(override, alias); err != nil {
		t.Fatal(err)
	}
	withOverride := func(name string) (string, bool) {
		if name == "MD_VIEW_DATA_DIR" {
			return alias, true
		}
		return "", false
	}
	got, err = resolveDataDir(withOverride, executable, executable)
	if err != nil {
		t.Fatalf("explicit data lookup failed: %v", err)
	}
	want, _ = canonicalPath(override)
	if got != want {
		t.Fatalf("explicit data dir = %q, want %q", got, want)
	}

	empty := func(string) (string, bool) { return "", true }
	if _, err := resolveDataDir(empty, executable, executable); err == nil || err.Error() != "MD_VIEW_DATA_DIR is empty" {
		t.Fatalf("empty MD_VIEW_DATA_DIR error = %v", err)
	}

	missing := func(string) (string, bool) { return filepath.Join(root, "missing"), true }
	if _, err := resolveDataDir(missing, executable, executable); err == nil || !strings.Contains(err.Error(), "missing Pandoc assets") {
		t.Fatalf("missing explicit assets error = %v", err)
	}
}

func TestValidateInputAndResolveOutput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	outputDir := filepath.Join(root, "output")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(source, "note.MD")
	writeFile(t, input, "# note\n")
	inputAlias := filepath.Join(root, "alias.md")
	if err := os.Symlink(input, inputAlias); err != nil {
		t.Fatal(err)
	}
	canonicalInput, err := validateInput(inputAlias)
	if err != nil {
		t.Fatalf("validateInput(alias) failed: %v", err)
	}
	wantInput, _ := canonicalPath(input)
	if canonicalInput != wantInput {
		t.Fatalf("canonical input = %q, want %q", canonicalInput, wantInput)
	}

	output, err := resolveExplicitOutput(filepath.Join(outputDir, "preview.html"), canonicalInput)
	if err != nil {
		t.Fatalf("resolveExplicitOutput failed: %v", err)
	}
	wantOutput, _ := canonicalPath(outputDir)
	wantOutput = filepath.Join(wantOutput, "preview.html")
	if output != wantOutput {
		t.Fatalf("output = %q, want %q", output, wantOutput)
	}

	if _, err := resolveExplicitOutput(inputAlias, canonicalInput); err == nil || !strings.Contains(err.Error(), "must not resolve") {
		t.Fatalf("input alias output error = %v", err)
	}
	outputAlias := filepath.Join(root, "output-alias.html")
	if err := os.Symlink(input, outputAlias); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveExplicitOutput(outputAlias, canonicalInput); err == nil || !strings.Contains(err.Error(), "must not resolve") {
		t.Fatalf("symlink output error = %v", err)
	}

	directoryOutput := filepath.Join(outputDir, "subdirectory")
	if err := os.Mkdir(directoryOutput, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveExplicitOutput(directoryOutput, canonicalInput); err == nil || !strings.Contains(err.Error(), "output path is a directory") {
		t.Fatalf("directory output error = %v", err)
	}
	if _, err := resolveExplicitOutput(filepath.Join(root, "missing", "preview.html"), canonicalInput); err == nil || !strings.Contains(err.Error(), "output directory does not exist") {
		t.Fatalf("missing output directory error = %v", err)
	}
}

func TestResolveCachedOutput(t *testing.T) {
	root := t.TempDir()
	tmpRoot := filepath.Join(root, "temporary root")
	if err := os.Mkdir(tmpRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "temporary-alias")
	if err := os.Symlink(tmpRoot, alias); err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) (string, bool) {
		if name == "TMPDIR" {
			return alias, true
		}
		return "", false
	}
	inputPath := filepath.Join(root, "source.md")
	got, err := resolveCachedOutput(lookup, inputPath)
	if err != nil {
		t.Fatalf("resolveCachedOutput failed: %v", err)
	}
	resolvedRoot, _ := canonicalPath(tmpRoot)
	want := filepath.Join(resolvedRoot, "md-view", fmt.Sprintf("md-view-%d.html", posixCksum([]byte(inputPath))))
	if got != want {
		t.Fatalf("cached output = %q, want %q", got, want)
	}
	if info, err := os.Stat(filepath.Dir(got)); err != nil || !info.IsDir() {
		t.Fatalf("cache directory was not created: info=%v err=%v", info, err)
	}
}

func TestMainRenderAtomicityAndBrowserGating(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.Mkdir(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dataDir, "pandoc"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "source.md")
	writeFile(t, source, "# source\n")
	outDir := filepath.Join(root, "output")
	if err := os.Mkdir(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(outDir, "preview.html")
	lookup := func(name string) (string, error) {
		return "/fake/" + name, nil
	}
	env := func(name string) (string, bool) {
		switch name {
		case "MD_VIEW_DATA_DIR":
			return dataDir, true
		case "TMPDIR":
			return filepath.Join(root, "tmp"), true
		default:
			return "", false
		}
	}
	if err := os.Mkdir(filepath.Join(root, "tmp"), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := &recordingRunner{}
	var stdout, stderr bytes.Buffer
	app := &App{
		Runner:         runner,
		LookPath:       lookup,
		LookupEnv:      env,
		ExecutablePath: filepath.Join(root, "missing-executable"),
		InvocationPath: "md-view",
		Program:        "md-view",
		Stdout:         &stdout,
		Stderr:         &stderr,
	}

	if status := app.Main(context.Background(), []string{"render", source, "--output", output}); status != 0 {
		t.Fatalf("successful render status = %d, stderr=%q", status, stderr.String())
	}
	canonicalOutputDir, _ := canonicalPath(outDir)
	resolvedOutput := filepath.Join(canonicalOutputDir, "preview.html")
	if got := stdout.String(); got != resolvedOutput+"\n" {
		t.Fatalf("render stdout = %q, want %q", got, resolvedOutput+"\n")
	}
	if got := string(readFile(t, output)); got != "rendered" {
		t.Fatalf("rendered output = %q, want rendered", got)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("render command count = %d, want 1", len(runner.commands))
	}
	command := runner.commands[0]
	resolvedDataDir, _ := canonicalPath(dataDir)
	resolvedSource, _ := canonicalPath(source)
	if command.Dir != resolvedDataDir {
		t.Fatalf("Pandoc working directory = %q, want %q", command.Dir, resolvedDataDir)
	}
	wantArgs := []string{
		"--defaults=" + filepath.Join(resolvedDataDir, "pandoc", "defaults.yaml"),
		"--resource-path=" + filepath.Dir(resolvedSource),
		"--output=" + command.Args[2][len("--output="):],
		resolvedSource,
	}
	if !equalStrings(command.Args, wantArgs) {
		t.Fatalf("Pandoc args = %#v, want %#v", command.Args, wantArgs)
	}
	if _, err := os.Stat(strings.TrimPrefix(command.Args[2], "--output=")); !os.IsNotExist(err) {
		t.Fatalf("temporary output still exists after rename: err=%v", err)
	}

	stdout.Reset()
	runner.commands = nil
	if status := app.Main(context.Background(), []string{source}); status != 0 {
		t.Fatalf("successful view status = %d, stderr=%q", status, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("view stdout = %q, want empty", stdout.String())
	}
	if len(runner.commands) != 2 || runner.commands[1].Name != "/fake/open" {
		t.Fatalf("view commands = %#v, want Pandoc then open", runner.commands)
	}
	expectedCache, err := resolveCachedOutput(env, resolvedSource)
	if err != nil {
		t.Fatal(err)
	}
	if runner.commands[1].Args[0] != expectedCache {
		t.Fatalf("open path = %q, want %q", runner.commands[1].Args[0], expectedCache)
	}

	writeFile(t, output, "sentinel")
	runner.commands = nil
	runner.failPandoc = true
	stderr.Reset()
	if status := app.Main(context.Background(), []string{"render", source, "--output", output}); status != 17 {
		t.Fatalf("failed Pandoc status = %d, want 17", status)
	}
	if got := string(readFile(t, output)); got != "sentinel" {
		t.Fatalf("failed render replaced output with %q", got)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("failed render command count = %d, want 1", len(runner.commands))
	}
	if !strings.Contains(stderr.String(), "fake pandoc failure") {
		t.Fatalf("Pandoc stderr = %q, want passthrough", stderr.String())
	}
	if strings.Contains(stderr.String(), "md-view: error:") {
		t.Fatalf("Pandoc failure received an extra md-view prefix: %q", stderr.String())
	}
	if tempPath := strings.TrimPrefix(runner.commands[0].Args[2], "--output="); fileExists(tempPath) {
		t.Fatalf("failed render left temporary output %q", tempPath)
	}
}

func TestMainMissingCommandAndOpenFailure(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "pandoc"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "source.md")
	writeFile(t, source, "source")
	outputDir := filepath.Join(root, "output")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmpDir := filepath.Join(root, "tmp")
	if err := os.Mkdir(tmpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lookupEnv := func(name string) (string, bool) {
		switch name {
		case "MD_VIEW_DATA_DIR":
			return dataDir, true
		case "TMPDIR":
			return tmpDir, true
		default:
			return "", false
		}
	}

	var stdout, stderr bytes.Buffer
	missingOpen := &App{
		Runner: &recordingRunner{},
		LookPath: func(name string) (string, error) {
			if name == "pandoc" {
				return "/fake/pandoc", nil
			}
			return "", errors.New("not found")
		},
		LookupEnv:      lookupEnv,
		ExecutablePath: filepath.Join(root, "missing-executable"),
		InvocationPath: "md-view",
		Program:        "md-view",
		Stdout:         &stdout,
		Stderr:         &stderr,
	}
	if status := missingOpen.Main(context.Background(), []string{source}); status != 1 {
		t.Fatalf("missing open status = %d, want 1", status)
	}
	if got := stderr.String(); !strings.Contains(got, "md-view: error: required command not found: open\n") {
		t.Fatalf("missing open stderr = %q", got)
	}

	runner := &recordingRunner{failOpen: true}
	stdout.Reset()
	stderr.Reset()
	app := *missingOpen
	app.Runner = runner
	app.LookPath = func(name string) (string, error) { return "/fake/" + name, nil }
	output := filepath.Join(outputDir, "preview.html")
	if status := app.Main(context.Background(), []string{"render", source, "--output", output}); status != 0 {
		t.Fatalf("render setup status = %d, stderr=%q", status, stderr.String())
	}
	// A view invocation now exercises the open process failure without
	// involving a failed render or a second output replacement.
	runner.commands = nil
	if status := app.Main(context.Background(), []string{source}); status != 19 {
		t.Fatalf("open failure status = %d, want 19", status)
	}
	if len(runner.commands) != 2 {
		t.Fatalf("open failure command count = %d, want 2", len(runner.commands))
	}
}

type recordingRunner struct {
	commands   []Command
	failPandoc bool
	failOpen   bool
}

func (r *recordingRunner) Run(_ context.Context, command Command) error {
	command.Args = append([]string(nil), command.Args...)
	r.commands = append(r.commands, command)
	if command.Name == "/fake/pandoc" {
		if r.failPandoc {
			if command.Stderr != nil {
				_, _ = io.WriteString(command.Stderr, "fake pandoc failure\n")
			}
			return commandExitError(17)
		}
		output := strings.TrimPrefix(command.Args[2], "--output=")
		writeFileToRunner(output, "rendered")
		return nil
	}
	if r.failOpen {
		return commandExitError(19)
	}
	return nil
}

func commandExitError(status int) error {
	return fakeExitError{status: status}
}

type fakeExitError struct {
	status int
}

func (e fakeExitError) Error() string { return fmt.Sprintf("exit status %d", e.status) }
func (e fakeExitError) ExitCode() int { return e.status }

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFileToRunner(path, contents string) {
	_ = os.WriteFile(path, []byte(contents), 0o600)
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
