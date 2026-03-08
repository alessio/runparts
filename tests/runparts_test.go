package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var (
	testBinary  string
	projectRoot string
)

func TestMain(m *testing.M) {
	// Resolve project root relative to this source file so that tests
	// work regardless of the working directory.
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot = filepath.Dir(filepath.Dir(thisFile))

	tmp, err := os.MkdirTemp("", "runparts-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	testBinary = filepath.Join(tmp, "runparts")
	cmd := exec.Command("go", "build", "-o", testBinary, ".")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic("failed to build binary: " + string(out))
	}

	os.Exit(m.Run())
}

// runBinary executes the test binary with the given args and stdin,
// returning stdout, stderr, and the exit code.
func runBinary(t *testing.T, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(testBinary, args...)
	cmd.Dir = projectRoot
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("unexpected error running binary: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// TestTestdata drives the tests from the testdata/TestCase* fixtures,
// mirroring run-tests.sh exactly. Each test case directory contains:
//   - args:       command-line arguments
//   - scripts/:   the scripts directory passed as the positional arg
//   - wantExit:   expected exit code
//   - wantStdout: expected stdout
//   - wantStderr: expected stderr
//
// The shell runner feeds the AUTHORS file as stdin; we do the same.
func TestTestdata(t *testing.T) {
	stdinContent, err := os.ReadFile(filepath.Join(projectRoot, "AUTHORS"))
	if err != nil {
		t.Fatal(err)
	}

	entries, err := filepath.Glob(filepath.Join(projectRoot, "testdata", "TestCase*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no TestCase* directories found in testdata")
	}

	for _, tcDir := range entries {
		name := filepath.Base(tcDir)
		t.Run(name, func(t *testing.T) {
			argsBytes, err := os.ReadFile(filepath.Join(tcDir, "args"))
			if err != nil {
				t.Fatal(err)
			}
			wantExitBytes, err := os.ReadFile(filepath.Join(tcDir, "wantExit"))
			if err != nil {
				t.Fatal(err)
			}
			wantStdout, err := os.ReadFile(filepath.Join(tcDir, "wantStdout"))
			if err != nil {
				t.Fatal(err)
			}
			wantStderr, err := os.ReadFile(filepath.Join(tcDir, "wantStderr"))
			if err != nil {
				t.Fatal(err)
			}

			wantExit, err := strconv.Atoi(strings.TrimSpace(string(wantExitBytes)))
			if err != nil {
				t.Fatalf("invalid wantExit: %v", err)
			}

			// Build the command args matching the shell runner:
			// runparts <args> testdata/<name>/scripts
			scriptsDir := filepath.Join("testdata", name, "scripts")
			var cmdArgs []string
			if argStr := strings.TrimSpace(string(argsBytes)); argStr != "" {
				cmdArgs = strings.Fields(argStr)
			}
			cmdArgs = append(cmdArgs, scriptsDir)

			cmd := exec.Command(testBinary, cmdArgs...)
			cmd.Dir = projectRoot
			cmd.Stdin = strings.NewReader(string(stdinContent))
			var outBuf, errBuf strings.Builder
			cmd.Stdout = &outBuf
			cmd.Stderr = &errBuf

			runErr := cmd.Run()
			gotExit := 0
			if runErr != nil {
				if ee, ok := runErr.(*exec.ExitError); ok {
					gotExit = ee.ExitCode()
				} else {
					t.Fatalf("unexpected error: %v", runErr)
				}
			}

			if gotExit != wantExit {
				t.Errorf("exit code: got %d, want %d", gotExit, wantExit)
			}
			if outBuf.String() != string(wantStdout) {
				t.Errorf("stdout mismatch\ngot:\n%s\nwant:\n%s", outBuf.String(), wantStdout)
			}
			if errBuf.String() != string(wantStderr) {
				t.Errorf("stderr mismatch\ngot:\n%s\nwant:\n%s", errBuf.String(), wantStderr)
			}
		})
	}
}

// makeScriptsDir creates a temp directory with executable scripts.
func makeScriptsDir(t *testing.T, scripts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range scripts {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const scriptOK = `#!/bin/sh
progname="$(basename ${0})"
echo "${progname}: STDOUT"
echo "${progname}: STDERR" >&2
`

const scriptFail = `#!/bin/sh
progname="$(basename ${0})"
echo "${progname}: STDOUT"
echo "${progname}: STDERR" >&2
exit 1
`

const scriptEchoArgs = `#!/bin/sh
echo "$@"
`

func TestReverse(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptOK,
		"02-script": scriptOK,
		"03-script": scriptOK,
	})

	stdout, _, exitCode := runBinary(t, "", "--reverse", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	wantStdout := "03-script: STDOUT\n02-script: STDOUT\n01-script: STDOUT\n"
	if stdout != wantStdout {
		t.Errorf("stdout mismatch\ngot:  %q\nwant: %q", stdout, wantStdout)
	}
}

func TestVerbose(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptOK,
	})

	_, stderr, exitCode := runBinary(t, "", "-v", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stderr, "executing") {
		t.Errorf("expected stderr to contain 'executing', got %q", stderr)
	}
	if !strings.Contains(stderr, "01-script") {
		t.Errorf("expected stderr to contain '01-script', got %q", stderr)
	}
}

func TestExitOnError(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptFail,
		"02-script": scriptOK,
	})

	stdout, _, exitCode := runBinary(t, "", "--exit-on-error", dir)
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if strings.Contains(stdout, "02-script") {
		t.Error("02-script should not have run after 01-script failed")
	}
}

func TestExitOnErrorContinuesWithoutFlag(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptFail,
		"02-script": scriptOK,
	})

	stdout, _, exitCode := runBinary(t, "", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (errors are logged but should not cause exit)", exitCode)
	}
	if !strings.Contains(stdout, "02-script: STDOUT") {
		t.Error("02-script should have run despite 01-script failing")
	}
}

func TestPassArguments(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptEchoArgs,
	})

	stdout, _, exitCode := runBinary(t, "", "-a", "hello", "-a", "world", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if strings.TrimSpace(stdout) != "hello world" {
		t.Errorf("expected 'hello world', got %q", strings.TrimSpace(stdout))
	}
}

func TestVerboseWithArgs(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptEchoArgs,
	})

	_, stderr, exitCode := runBinary(t, "", "-v", "-a", "foo", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stderr, "executing") || !strings.Contains(stderr, "foo") {
		t.Errorf("expected verbose output with args, got %q", stderr)
	}
}

func TestMissingOperand(t *testing.T) {
	_, _, exitCode := runBinary(t, "")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for missing operand")
	}
}

func TestListAndTestMutuallyExclusive(t *testing.T) {
	_, _, exitCode := runBinary(t, "", "--list", "--test", "/tmp")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for --list and --test together")
	}
}

func TestNonexistentDirectory(t *testing.T) {
	_, _, exitCode := runBinary(t, "", "/nonexistent/directory/path")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for nonexistent directory")
	}
}

func TestEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	_, _, exitCode := runBinary(t, "", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for empty directory, got %d", exitCode)
	}
}

func TestInvalidFilenamesSkipped(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"valid-script": scriptOK,
		"has.dot":      scriptOK,
		"has space":    scriptOK,
	})

	stdout, _, exitCode := runBinary(t, "", "--test", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if !strings.HasSuffix(lines[0], "valid-script") {
		t.Errorf("expected valid-script, got %q", lines[0])
	}
}

func TestVersion(t *testing.T) {
	stdout, _, exitCode := runBinary(t, "", "-V")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "runparts program") {
		t.Errorf("expected version output, got %q", stdout)
	}
}

func TestHelp(t *testing.T) {
	stdout, _, exitCode := runBinary(t, "", "-h")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("expected usage output, got %q", stdout)
	}
}

func TestUmask(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptOK,
	})

	_, _, exitCode := runBinary(t, "", "--umask", "077", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

func TestDirectoriesSkipped(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptOK,
	})
	if err := os.Mkdir(filepath.Join(dir, "02-subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runBinary(t, "", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if strings.Contains(stdout, "02-subdir") {
		t.Error("subdirectory should have been skipped")
	}
	if !strings.Contains(stdout, "01-script: STDOUT") {
		t.Error("01-script should have run")
	}
}

func TestCustomRegexExecution(t *testing.T) {
	dir := makeScriptsDir(t, map[string]string{
		"01-script": scriptOK,
		"02-script": scriptOK,
		"03-script": scriptOK,
	})

	stdout, _, exitCode := runBinary(t, "", "--regex=^0[13]-", "--", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	wantStdout := "01-script: STDOUT\n03-script: STDOUT\n"
	if stdout != wantStdout {
		t.Errorf("stdout mismatch\ngot:  %q\nwant: %q", stdout, wantStdout)
	}
}
