package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"al.essio.dev/cmd/runparts/internal/version"

	flag "github.com/hitbros/pflag"

	"golang.org/x/sys/unix"
)

const defaultUmask = "022"

var (
	exitOnErrorMode bool
	filenameRegex   string
	helpMode        bool
	listMode        bool
	lsbsysinitMode  bool
	reportMode      bool
	reverseMode     bool
	stdinMode       bool
	testMode        bool
	umask           string
	verboseMode     bool
	versionMode     bool

	regexes    []*regexp.Regexp
	scriptArgs []string
	stdinCopy  *os.File

	binBasename = ""
)

func init() {
	binBasename = filepath.Base(os.Args[0])

	flag.BoolVar(&listMode, "list", false, "print names of all valid files (can not be used with --test)")
	flag.BoolVar(&testMode, "test", false, "print script names which would run, but don't run them.")
	flag.BoolVar(&reportMode, "report", false, "print script names if they produce output.")
	flag.BoolVar(&reverseMode, "reverse", false, "reverse the scripts' execution order.")
	flag.BoolVar(&stdinMode, "stdin", false, "multiplex stdin to scripts being run, using temporary file.")
	flag.BoolVar(&exitOnErrorMode, "exit-on-error", false, "exit as soon as a script returns with a non-zero exit code.")
	flag.BoolVarP(&verboseMode, "verbose", "v", false, "print script names before running them.")
	flag.BoolVarP(&versionMode, "version", "V", false, "output version information and exit.")
	flag.StringSliceVarP(&scriptArgs, "arg", "a", []string{}, "pass ARGUMENT to scripts, use once for each argument.")
	flag.BoolVarP(&helpMode, "help", "h", false, "display this help and exit.")
	flag.StringVarP(&umask, "umask", "u", defaultUmask, "sets umask to UMASK (octal).")
	flag.StringVar(&filenameRegex, "regex", "", "validate filenames based on POSIX ERE pattern PATTERN.")
	flag.BoolVar(&lsbsysinitMode, "lsbsysinit", false, "validate filenames based on LSB sysinit specs.")
	flag.Usage = usage
	flag.ErrHelp = nil

	flag.CommandLine.SetOutput(os.Stderr)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(fmt.Sprintf("%s: ", binBasename))
	log.SetOutput(os.Stderr)
	flag.Parse()

	if helpMode {
		usage()
		return
	}

	if versionMode {
		printVersion()
		return
	}

	validateArgs()
	dirname := flag.Arg(0)
	filenames, err := listDirectory(dirname, reverseMode)
	if err != nil {
		log.Fatalf("failed to open directory %s: %v", dirname, err)
	}

	setUmask(umask)
	if err := runParts(dirname, filenames, scriptArgs, isValidName(regexes), testMode, listMode, verboseMode, exitOnErrorMode, stdinMode); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Printf("Usage: %s [OPTION]... DIRECTORY\n", binBasename)
	fmt.Print(flag.CommandLine.FlagUsages())
}

func printVersion() {
	fmt.Println("alessio's runparts program, version", version.Version)
	fmt.Println("Copyright (C) 2020-2024 Alessio Treglia <alessio@debian.org>")
}

func validateArgs() {
	if flag.NArg() != 1 {
		log.Fatal("missing operand")
	}

	if listMode && testMode {
		log.Fatal("-list and -test can not be used together")
	}

	switch {
	case filenameRegex != "":
		regex, err := regexp.Compile(filenameRegex)
		if err != nil {
			log.Fatalf("failed to compile regular expression: %v", err)
		}
		regexes = []*regexp.Regexp{regex}
	case lsbsysinitMode:
		regexes = []*regexp.Regexp{
			regexp.MustCompile("^_?([a-z0-9_.]+-)+[a-z0-9]+$"),
			regexp.MustCompile(`^[a-z0-9-].*\.dpkg-(old|dist|new|tmp)$`),
			regexp.MustCompile("^[a-z0-9][a-z0-9-]*$"),
		}
	default:
		regexes = []*regexp.Regexp{regexp.MustCompile("^[a-zA-Z0-9_-]+$")}
	}
}

func listDirectory(targetDir string, reverseOrder bool) ([]string, error) {
	f, err := os.Open(targetDir)
	if err != nil {
		return nil, err
	}

	filenames, err := f.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	if reverseOrder {
		sort.Sort(sort.Reverse(sort.StringSlice(filenames)))
	} else {
		sort.Sort(sort.StringSlice(filenames))
	}

	return filenames, nil
}

func setUmask(s string) {
	mask, err := strconv.ParseUint(s, 8, 16)
	if err != nil {
		log.Fatal(err)
	}
	if mask > 07777 {
		log.Fatal("bad umask value")
	}

	_ = syscall.Umask(int(mask))
}

func runParts(dirname string, filenames []string, scriptArgs []string,
	isValidNameFunc func(string) bool,
	testMode, listMode, verboseMode, exitOnErrorMode, stdinMode bool) error {

	if len(filenames) == 0 {
		return nil
	}

	var err error
	if stdinMode {
		stdinCopy, err = copyStdin()
		if stdinCopy != nil {
			defer func() {
				if err := os.Remove(stdinCopy.Name()); err != nil {
					log.Println(fmt.Errorf("couldn't remove file %q: %w", stdinCopy.Name(), err))
				}
			}()
		}
		if err != nil {
			return err
		}
	}

	for _, fname := range filenames {
		if !isValidNameFunc(fname) {
			continue
		}
		filename := filepath.Join(dirname, fname)
		fileinfo, err := os.Stat(filename)
		if err != nil {
			err2 := fmt.Errorf("failed to stat component %s: %v", filename, err)
			if exitOnErrorMode {
				return err2
			}
			log.Print(err2.Error())
		}

		mode := fileinfo.Mode()
		if mode.IsDir() {
			continue
		}
		if !mode.IsRegular() {
			log.Printf("component %s is not an executable plain file", filename)
			continue
		}

		if err := unix.Access(filename, unix.X_OK); err == nil {
			switch {
			case testMode:
				fmt.Println(filename)
			case listMode:
				if err := unix.Access(filename, unix.R_OK); err == nil {
					fmt.Println(filename)
				}
			default:
				if verboseMode {
					if len(scriptArgs) == 0 {
						log.Printf("executing %s", filename)
					} else {
						log.Printf("executing %s %s", filename, strings.Join(scriptArgs, " "))
					}
				}

				var err error
				if stdinMode {
					stdinCopy.Seek(0, 0)
					err = runPart(filename, stdinCopy, scriptArgs)
				} else {
					err = runPart(filename, os.Stdin, scriptArgs)
				}

				if err != nil && exitOnErrorMode {
					return formatExitError(filename, err)
				} else if err != nil {
					log.Println(formatExitError(filename, err))
				}
			}
			continue
		} else if err := unix.Access(filename, unix.R_OK); err == nil && listMode {
			fmt.Println(filename)
		}

		if err := unix.Access(filename, unix.R_OK); err != nil && listMode {
			fmt.Println(filename)
		} else if mode&os.ModeSymlink != 0 && !listMode {
			return fmt.Errorf("component %s is a broken symbolic link", filename)
		}

		continue

	}

	return nil
}

func runPart(filename string, input io.Reader, args []string) error {
	cmd := exec.Command(filename, args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Stdin = input
	if err := cmd.Start(); err != nil {
		return err
	}

	errSlurp, err := io.ReadAll(stderr)
	if err != nil {
		log.Fatal(err)
	}

	outSlurp, err := io.ReadAll(stdout)
	if err != nil {
		log.Fatal(err)
	}

	if reportMode && (len(errSlurp) != 0 || len(outSlurp) != 0) {
		fmt.Printf("%s:\n", filename)
	}

	fmt.Fprintf(os.Stderr, "%s", errSlurp)
	fmt.Fprintf(os.Stdout, "%s", outSlurp)

	return cmd.Wait()
}

func isValidName(exprs []*regexp.Regexp) func(name string) bool {
	return func(name string) bool {
		if len(exprs) == 0 {
			return true
		}
		for _, r := range exprs {
			if r.MatchString(name) {
				return true
			}
		}
		return false
	}
}

func formatExitError(filename string, err error) error {
	var eerr *exec.ExitError
	if errors.As(err, &eerr) {
		return fmt.Errorf("%s exited with return code %d", filename, eerr.ExitCode())
	}
	return fmt.Errorf("failed to exec %s: %s", filename, err.Error())
}

func copyStdin() (*os.File, error) {
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return nil, fmt.Errorf("couldn't create temporary file: %v", err)
	}

	if _, err := io.Copy(tmp, os.Stdin); err != nil {
		return tmp, fmt.Errorf("couldn't copy stdin: %v", err)
	}

	return tmp, nil
}
