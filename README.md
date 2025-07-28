# Watcher

Watcher is a simple command line tool that runs some command if it identifies changes in a set of files. It is useful for tasks like running tests, building projects, or any other command that needs to be executed when files change, especially if the command is meant to be run repeatedly.

## Installation

You can install watcher using the `go install` command:

    go install github.com/alan-b-lima/watcher@latest

## Usage

    watcher --help
    watcher --version
    watcher { <filepath> } { <option> } ( --exec | -e ) <command> [ <args> ]

### Directives
    
    <filepath>     - path to a file or directory.
    <option>       - options to be passed to the watcher.
    <command>      - any command.
    <args>         - arguments to be passed to the command.
    <milliseconds> - number of milliseconds.

### Options

    --help | -h                          - displays this screen.
    --version | -v                       - displays the version of the application.
    ( --watch | -w ) { <filename> }      - adds more filepaths to watch.
    ( --ignore | -i ) { <filename> }     - skips watching the filepaths given after this flag.
    ( --tick-speed | -t ) <milliseconds> - defines the wait time in between watches.
    ( --exec | -e ) <command> [ <args> ] - command to be executed when changes are detected.


## Examples

    watcher . --ignore .git node_modules .gitignore -t 1000 --exec build.sh

Watches for changes every second (-t 1000) on the current directory (.), except for the .git and node_modules directories and the .gitignore file. If any changes are detected, build.sh is run and its output will be displayed on the standard output.

    watcher src --tick-speed 3000 -e "go test -v ./..."

Watches for changes every three second (--tick-speed 3000) in the src directory and runs `go test -v ./...` whenever a change is detected.