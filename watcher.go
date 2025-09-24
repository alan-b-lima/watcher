package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alan-b-lima/watcher/ansi-escape"
)

const (
	Granularity = 100 * time.Millisecond
	Version     = "v0.0.3"
)

const (
	flagWatch = iota
	flagIgnore
	flagExec
	flagTickSpeed
	flagAfterTickSpeed
)

var flags = map[string]int{
	"-w": flagWatch, "--watch": flagWatch,
	"-i": flagIgnore, "--ignore": flagIgnore,
	"-e": flagExec, "--exec": flagExec,
	"-t": flagTickSpeed, "--tick-speed": flagTickSpeed,
}

var (
	errNothingToWatchOver        = errors.New("no file to watch over has been given")
	errNoExecFlag                = errors.New("no execution flag has been found or there is nothing after it")
	errUnknownFlag               = func(flag string) error { return fmt.Errorf("unknown flag: %s", flag) }
	errArgAfterTickSpeedFlag     = errors.New("only one argument should be passed after a tick speed flag")
	errFailedToParseMilliseconds = errors.New("given milliseconds failed to be parsed as a number")
	errTickSpeedNonPositive      = errors.New("tick speed must be positive")
	errTickSpeedGranAlreadySet   = errors.New("the tick speed has already been set")
	errUnsupportedOS             = func(os string) error { return unsupportedOSError{fmt.Errorf("unsupported OS: %s", os)} }
)

type flagState struct {
	watch  []string
	ignore []string
	gran   time.Duration
	exec   []string
}

type unsupportedOSError struct {
	error
}

type startProcessFailureError struct {
	error
}

func main() {
	if err := ansi.EnableVirtualTerminal(os.Stdout.Fd()); err != nil {
		fmt.Println("failed to enable virtual terminal:", err)
		return
	}
	defer ansi.DisableVirtualTerminal(os.Stdout.Fd())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer close(signals)

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "--help", "-h":
			help()
			return

		case "--version", "-v":
			version()
			return
		}
	}

	fls, err := processFlags(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		return
	}

	currentLatestModTime, _, err := fls.findLatestChange()
	if err != nil {
		fmt.Println(err)
		return
	}

	if ok := fls.executeAndHandle(""); !ok {
		return
	}

	ticker := time.NewTicker(fls.gran)
	defer ticker.Stop()

	for {
		select {
		case <-signals:
			return

		case <-ticker.C:
			latestModTime, filename, err := fls.findLatestChange()
			if err != nil {
				fmt.Println(err)
				return
			}

			if !latestModTime.After(currentLatestModTime) {
				fmt.Printf("[\033[90m%s\033[m]\r", time.Now().Format(time.DateTime))
				continue
			}

			if ok := fls.executeAndHandle(filename); !ok {
				return
			}

			currentLatestModTime = latestModTime
		}
	}
}

func processFlags(args []string) (flagState, error) {
	var fls flagState

	currentFlag := flagWatch
	for i, arg := range args {
		flag, ok := flags[arg]
		if ok {
			currentFlag = flag
			continue
		}

		if isFlagLike(arg) {
			return flagState{}, errUnknownFlag(arg)
		}

		switch currentFlag {
		case flagWatch:
			fls.watch = append(fls.watch, arg)

		case flagIgnore:
			fls.ignore = append(fls.ignore, arg)

		case flagExec:
			fls.exec = args[i:]
			goto exit

		case flagTickSpeed:
			num, err := strconv.ParseInt(arg, 10, 0)
			if err != nil {
				return flagState{}, errFailedToParseMilliseconds
			}

			if num <= 0 {
				return flagState{}, errTickSpeedNonPositive
			}

			if fls.gran != time.Duration(0) {
				return flagState{}, errTickSpeedGranAlreadySet
			}

			fls.gran = time.Duration(num).Round(Granularity)
			currentFlag = flagAfterTickSpeed

		case flagAfterTickSpeed:
			return flagState{}, errArgAfterTickSpeedFlag

		}
	}

	return flagState{}, errNoExecFlag

exit:
	if len(fls.watch) == 0 {
		return flagState{}, errNothingToWatchOver
	}

	if fls.gran == time.Duration(0) {
		fls.gran = Granularity
	}

	if err := fls.normalizePaths(); err != nil {
		return flagState{}, err
	}

	return fls, nil
}

func isFlagLike(arg string) bool {
	return strings.HasPrefix(arg, "-")
}

func (fls *flagState) normalizePaths() error {
	for i := range len(fls.watch) {
		result, err := filepath.Abs(fls.watch[i])
		if err != nil {
			return err
		}

		fls.watch[i] = result
	}

	for i := range len(fls.ignore) {
		fls.ignore[i] = filepath.Clean(fls.ignore[i])

		if _, err := filepath.Match(fls.ignore[i], ""); err != nil {
			return err
		}
	}

	return nil
}

func (fls *flagState) selectiveWalk(action func(string, fs.FileInfo) error) error {
	for _, root := range fls.watch {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			for _, ig := range fls.ignore {
				if strings.HasSuffix(path, ig) {
					return filepath.SkipDir
				}

				if match, _ := filepath.Match(ig, strings.ReplaceAll(path, "\\", "/")); match {
					return filepath.SkipDir
				}
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			return action(path, info)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (fls *flagState) findLatestChange() (latestModTime time.Time, filename string, err error) {
	err = fls.selectiveWalk(func(path string, info fs.FileInfo) error {
		modTime := info.ModTime()
		if modTime.After(latestModTime) {
			latestModTime = modTime
			filename = path
		}

		return nil
	})

	return latestModTime, filename, err
}

func (fls *flagState) executeAndHandle(filename string) bool {
	if filename == "" {
		fmt.Printf("\033[2J\033[1;1H[\033[90m%s\033[m] First execution\033[m\n\n", time.Now().Format(time.DateTime))
	} else {
		fmt.Printf("\033[2J\033[1;1H[\033[90m%s\033[m] %s has changed\033[m\n\n", time.Now().Format(time.DateTime), filename)
	}

	err := fls.execute()
	switch err := err.(type) {

	case *unsupportedOSError:
		fmt.Println(err)
		return false

	case *startProcessFailureError:
		fmt.Println(err)
		return false

	case *exec.ExitError:
		if code := err.ExitCode(); code != 0 {
			fmt.Printf("\nexited with code \033[33m%d\033[m\n", code)
		}
	}

	fmt.Print("\n")
	return true
}

func (fls *flagState) execute() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", append([]string{"/c"}, fls.exec...)...)
	case "darwin", "linux":
		cmd = exec.Command("/bin/sh", "-c", strings.Join(fls.exec, " "))
	default:
		return errUnsupportedOS(runtime.GOOS)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

func help() {
	version()
	fmt.Println(helpString)
}

func version() {
	fmt.Printf("watcher %s for %s\n", Version, runtime.GOOS)
}

const helpString = `
synopsis:
    watcher --help
    watcher --version
    watcher { <filepath> } { <option> } ( --exec | -e ) <command> [ <args> ]

description:
    watches for changes on the given files and directories (and files inside the given directories)
    over a period of time and runs the given command whenever any changes are detected.

directives:
    <filepath>     - path to a file or directory.
    <command>      - any command.
    <args>         - arguments to be passed to the command.
    <milliseconds> - number of milliseconds.
    
    options:
        --help | -h                          - displays this screen.
    	--version | -v                       - displays the version of the application.
    	( --watch | -w ) { <filename> }      - adds more filepaths to watch.
    	( --ignore | -i ) { <filename> }     - skips watching the filepaths given after this flag.
    	( --tick-speed | -t ) <milliseconds> - defines the wait time in between watches.
    	( --exec | -e ) <command> [ <args> ] - command to be executed when changes are detected.

example:
    watcher . --ignore .git node_modules .gitignore -t 1000 --exec build.sh

    	watches over changes every second (-t 1000) on the current directory (.), except for the
		.git and node_modules directories and the .gitignore file. If any changes are detected,
		build.sh is run and its output will be displayed on the standard output.
		
	watcher src --tick-speed 3000 -e "go test -v ./..."

		watches for changes every three second (--tick-speed 3000) in the src directory and runs go
		test -v ./... whenever a change is detected.`
