package mdview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	programName = "md-view"

	usageText = `Usage:
  md-view FILE.md
  md-view render FILE.md --output OUTPUT.html

Render a Markdown file to standalone HTML and open it in the default browser.
The render subcommand writes HTML without opening a browser.
`

	helpText = `Usage:
  md-view FILE.md
  md-view render FILE.md --output OUTPUT.html

The default command renders to a deterministic cache file below TMPDIR and
opens it with macOS open. The render subcommand requires an existing output
directory and never opens a browser. -o and --output=PATH are also accepted.
`
)

type Mode uint8

const (
	ViewMode Mode = iota
	RenderMode
)

type Options struct {
	Mode   Mode
	Input  string
	Output string
	Help   bool
}

type usageError struct {
	message string
}

func (e *usageError) Error() string { return e.message }

// Command describes one external process invocation.
type Command struct {
	Name   string
	Args   []string
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
}

type Runner interface {
	Run(context.Context, Command) error
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, command Command) error {
	process := exec.CommandContext(ctx, command.Name, command.Args...)
	process.Dir = command.Dir
	process.Stdout = command.Stdout
	process.Stderr = command.Stderr
	return process.Run()
}

type App struct {
	Runner Runner

	// LookPath and LookupEnv are injectable so command and environment
	// behavior can be tested without changing the production process.
	LookPath  func(string) (string, error)
	LookupEnv func(string) (string, bool)

	ExecutablePath string
	InvocationPath string
	Program        string

	Stdout io.Writer
	Stderr io.Writer
}

func NewApp() *App {
	invocationPath := ""
	if len(os.Args) > 0 {
		invocationPath = os.Args[0]
	}
	executablePath, err := os.Executable()
	if err != nil || executablePath == "" {
		executablePath = invocationPath
	}

	program := filepath.Base(invocationPath)
	if program == "" || program == "." || program == string(filepath.Separator) {
		program = programName
	}

	return &App{
		Runner:         OSRunner{},
		LookPath:       exec.LookPath,
		LookupEnv:      os.LookupEnv,
		ExecutablePath: executablePath,
		InvocationPath: invocationPath,
		Program:        program,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
	}
}

func (a *App) Main(ctx context.Context, args []string) int {
	a.setDefaults()
	if ctx == nil {
		ctx = context.Background()
	}

	dataDir, err := resolveDataDir(a.LookupEnv, a.ExecutablePath, a.InvocationPath)
	if err != nil {
		a.printError(err)
		return 1
	}

	options, err := parseArgs(args)
	if err != nil {
		var usageErr *usageError
		if errors.As(err, &usageErr) {
			if usageErr.message != "" {
				fmt.Fprintf(a.stderr(), "%s: error: %s\n", a.program(), usageErr.message)
			}
			a.printUsage()
			return 2
		}
		a.printError(err)
		return 1
	}
	if options.Help {
		fmt.Fprint(a.stdout(), helpText)
		return 0
	}

	inputPath, err := validateInput(options.Input)
	if err != nil {
		a.printError(err)
		return 1
	}

	pandocPath, err := a.findCommand("pandoc")
	if err != nil {
		a.printError(err)
		return 1
	}

	var outputPath string
	if options.Mode == ViewMode {
		openPath, err := a.findCommand("open")
		if err != nil {
			a.printError(err)
			return 1
		}

		outputPath, err = resolveCachedOutput(a.LookupEnv, inputPath)
		if err != nil {
			a.printError(err)
			return 1
		}

		if err := a.render(ctx, dataDir, pandocPath, inputPath, outputPath); err != nil {
			return a.processOrPrintError(err)
		}
		if err := a.open(ctx, openPath, outputPath); err != nil {
			return a.processOrPrintError(err)
		}
		return 0
	}

	outputPath, err = resolveExplicitOutput(options.Output, inputPath)
	if err != nil {
		a.printError(err)
		return 1
	}
	if err := a.render(ctx, dataDir, pandocPath, inputPath, outputPath); err != nil {
		return a.processOrPrintError(err)
	}
	fmt.Fprintln(a.stdout(), outputPath)
	return 0
}

func (a *App) setDefaults() {
	if a.Runner == nil {
		a.Runner = OSRunner{}
	}
	if a.LookPath == nil {
		a.LookPath = exec.LookPath
	}
	if a.LookupEnv == nil {
		a.LookupEnv = os.LookupEnv
	}
	if a.Stdout == nil {
		a.Stdout = io.Discard
	}
	if a.Stderr == nil {
		a.Stderr = io.Discard
	}
	if a.Program == "" {
		a.Program = programName
	}
	if a.InvocationPath == "" {
		if len(os.Args) > 0 {
			a.InvocationPath = os.Args[0]
		}
	}
	if a.ExecutablePath == "" {
		a.ExecutablePath, _ = os.Executable()
		if a.ExecutablePath == "" {
			a.ExecutablePath = a.InvocationPath
		}
	}
}

func (a *App) program() string {
	if a.Program == "" {
		return programName
	}
	return a.Program
}

func (a *App) stdout() io.Writer { return a.Stdout }

func (a *App) stderr() io.Writer { return a.Stderr }

func (a *App) printUsage() { fmt.Fprint(a.stderr(), usageText) }

func (a *App) printError(err error) {
	fmt.Fprintf(a.stderr(), "%s: error: %s\n", a.program(), err)
}

func (a *App) processOrPrintError(err error) int {
	var processErr processError
	if errors.As(err, &processErr) {
		return processErr.status
	}
	a.printError(err)
	return 1
}

func (a *App) findCommand(name string) (string, error) {
	path, err := a.LookPath(name)
	if err != nil || path == "" {
		return "", fmt.Errorf("required command not found: %s", name)
	}
	return path, nil
}

func (a *App) render(ctx context.Context, dataDir, pandocPath, inputPath, outputPath string) error {
	temporary, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".tmp.")
	if err != nil {
		return fmt.Errorf("cannot create temporary output beside %s", outputPath)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Close(); err != nil {
		return fmt.Errorf("cannot create temporary output beside %s", outputPath)
	}

	command := Command{
		Name: pandocPath,
		Args: []string{
			"--defaults=" + filepath.Join(dataDir, "pandoc", "defaults.yaml"),
			"--resource-path=" + filepath.Dir(inputPath),
			"--output=" + temporaryPath,
			inputPath,
		},
		Dir:    dataDir,
		Stdout: a.stdout(),
		Stderr: a.stderr(),
	}
	if err := a.Runner.Run(ctx, command); err != nil {
		return processError{status: exitStatus(err)}
	}

	if err := os.Rename(temporaryPath, outputPath); err != nil {
		return fmt.Errorf("cannot install rendered HTML: %s", outputPath)
	}
	removeTemporary = false
	return nil
}

func (a *App) open(ctx context.Context, openPath, outputPath string) error {
	if err := a.Runner.Run(ctx, Command{
		Name:   openPath,
		Args:   []string{outputPath},
		Stdout: a.stdout(),
		Stderr: a.stderr(),
	}); err != nil {
		return processError{status: exitStatus(err)}
	}
	return nil
}

type processError struct {
	status int
}

func (e processError) Error() string { return "external command failed" }

func exitStatus(err error) int {
	var exitCoder interface{ ExitCode() int }
	if errors.As(err, &exitCoder) {
		if status := exitCoder.ExitCode(); status >= 0 {
			return status
		}
	}
	return 1
}

func parseArgs(args []string) (Options, error) {
	if len(args) == 0 {
		return Options{}, &usageError{}
	}

	switch args[0] {
	case "-h", "--help":
		if len(args) != 1 {
			return Options{}, &usageError{}
		}
		return Options{Help: true}, nil
	case "render":
		return parseRenderArgs(args[1:])
	default:
		if len(args) != 1 {
			return Options{}, &usageError{}
		}
		return Options{Mode: ViewMode, Input: args[0]}, nil
	}
}

func parseRenderArgs(args []string) (Options, error) {
	var input string
	var output string

	for index := 0; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--output":
			if index+1 >= len(args) {
				return Options{}, &usageError{}
			}
			if output != "" {
				return Options{}, &usageError{message: "multiple output paths"}
			}
			output = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--output="):
			if output != "" {
				return Options{}, &usageError{message: "multiple output paths"}
			}
			output = strings.TrimPrefix(argument, "--output=")
			index++
		case argument == "-o":
			if index+1 >= len(args) {
				return Options{}, &usageError{}
			}
			if output != "" {
				return Options{}, &usageError{message: "multiple output paths"}
			}
			output = args[index+1]
			index += 2
		case argument == "--":
			index++
			if len(args)-index != 1 {
				return Options{}, &usageError{}
			}
			if input != "" {
				return Options{}, &usageError{message: "multiple input files"}
			}
			input = args[index]
			index++
		case strings.HasPrefix(argument, "-"):
			return Options{}, &usageError{message: fmt.Sprintf("unknown option: %s", argument)}
		default:
			if input != "" {
				return Options{}, &usageError{message: "multiple input files"}
			}
			input = argument
			index++
		}
	}

	if input == "" {
		return Options{}, &usageError{message: "render requires an input Markdown file"}
	}
	if output == "" {
		return Options{}, &usageError{message: "render requires --output OUTPUT.html"}
	}
	return Options{Mode: RenderMode, Input: input, Output: output}, nil
}

func resolveDataDir(lookupEnv func(string) (string, bool), executablePath, invocationPath string) (string, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	if dataDir, set := lookupEnv("MD_VIEW_DATA_DIR"); set {
		if dataDir == "" {
			return "", errors.New("MD_VIEW_DATA_DIR is empty")
		}
		if !isDirectory(filepath.Join(dataDir, "pandoc")) {
			return "", fmt.Errorf("md-view data directory is missing Pandoc assets: %s", dataDir)
		}
		resolved, err := canonicalPath(dataDir)
		if err != nil {
			return "", fmt.Errorf("cannot access md-view data directory: %s", dataDir)
		}
		return resolved, nil
	}

	resolvedExecutable, err := canonicalPath(executablePath)
	if err != nil {
		return "", errors.New("cannot resolve md-view executable path")
	}
	executableDir := filepath.Dir(resolvedExecutable)
	repositoryDataDir := filepath.Clean(filepath.Join(executableDir, ".."))
	if isDirectory(filepath.Join(repositoryDataDir, "pandoc")) {
		resolved, err := canonicalPath(repositoryDataDir)
		if err != nil {
			return "", errors.New("cannot resolve md-view data directory")
		}
		return resolved, nil
	}

	installedDataDir := filepath.Clean(filepath.Join(executableDir, "..", "share", "md-view"))
	if isDirectory(filepath.Join(installedDataDir, "pandoc")) {
		resolved, err := canonicalPath(installedDataDir)
		if err != nil {
			return "", errors.New("cannot resolve installed md-view data directory")
		}
		return resolved, nil
	}

	displayPath := invocationPath
	if displayPath == "" {
		displayPath = executablePath
	}
	return "", fmt.Errorf("md-view Pandoc assets were not found beside %s", displayPath)
}

func validateInput(inputArg string) (string, error) {
	info, err := os.Stat(inputArg)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("input is not a regular file: %s", inputArg)
	}

	file, err := os.Open(inputArg)
	if err != nil {
		return "", fmt.Errorf("input is not readable: %s", inputArg)
	}
	_ = file.Close()

	inputPath, err := canonicalPath(inputArg)
	if err != nil {
		return "", fmt.Errorf("cannot resolve input path: %s", inputArg)
	}
	if !isMarkdownName(filepath.Base(inputPath)) {
		return "", fmt.Errorf("input must have a Markdown extension (.md or .markdown): %s", inputArg)
	}
	return inputPath, nil
}

func isMarkdownName(name string) bool {
	extension := name
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		extension = name[dot+1:]
	}
	switch strings.ToLower(extension) {
	case "md", "markdown", "mdown", "mkdn", "mkd", "mdwn":
		return true
	default:
		return false
	}
}

func resolveExplicitOutput(outputArg, inputPath string) (string, error) {
	if outputArg == "" {
		return "", errors.New("an output path is required")
	}

	outputParentArg := filepath.Dir(outputArg)
	if !isDirectory(outputParentArg) {
		return "", fmt.Errorf("output directory does not exist: %s", outputParentArg)
	}
	outputParent, err := canonicalPath(outputParentArg)
	if err != nil {
		return "", fmt.Errorf("cannot resolve output directory: %s", outputParentArg)
	}
	outputName := filepath.Base(outputArg)
	if outputName == "" {
		return "", fmt.Errorf("output path has no filename: %s", outputArg)
	}
	outputPath := filepath.Join(outputParent, outputName)

	outputResolved := outputPath
	if _, err := os.Lstat(outputArg); err == nil {
		outputResolved, err = canonicalPath(outputArg)
		if err != nil {
			return "", fmt.Errorf("cannot resolve output path: %s", outputArg)
		}
	}
	if outputResolved == inputPath {
		return "", errors.New("output path must not resolve to the input Markdown file")
	}
	if isDirectory(outputPath) {
		return "", fmt.Errorf("output path is a directory: %s", outputArg)
	}
	return outputPath, nil
}

func resolveCachedOutput(lookupEnv func(string) (string, bool), inputPath string) (string, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	tempRoot, set := lookupEnv("TMPDIR")
	if !set || tempRoot == "" {
		tempRoot = "/tmp"
	}
	if !isDirectory(tempRoot) {
		return "", fmt.Errorf("TMPDIR does not exist: %s", tempRoot)
	}
	resolvedTempRoot, err := canonicalPath(tempRoot)
	if err != nil {
		return "", fmt.Errorf("cannot resolve TMPDIR: %s", tempRoot)
	}
	cacheDir := filepath.Join(resolvedTempRoot, "md-view")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create md-view cache directory: %s", cacheDir)
	}
	return filepath.Join(cacheDir, fmt.Sprintf("md-view-%d.html", posixCksum([]byte(inputPath)))), nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func canonicalPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absPath)
}
