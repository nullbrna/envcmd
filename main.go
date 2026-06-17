package main

import (
    "bufio"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "sync"
)

const (
    ErrCurrentDirectory = "Failed to get current directory: %v"
    ErrWorkingBranch    = "Failed to get your branch (may not be in a repository): %v"
    ErrInvalidEntry     = "Bad format <%s>, expected EVC_[ASYNC_]<DIR|BRA>_<TARGET>"
    ErrRunningCommand   = "Failed to start/complete command <%s>: %v"
)

var (
    // Program version passed at build-time.
    version = "v0.0.0"
    // ANSI colour codes: Green, Yellow, Blue, Magenta, and Cyan.
    colours = []string{"32", "33", "34", "35", "36"}
    // Current directory and branch to run the commands against.
    directory, branch string
)

func logAndAbort(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "\x1b[1;31m[E]\x1b[0m "+format+"\n", args...)
    os.Exit(1)
}

func init() {
    working, err := os.Getwd()
    if err != nil {
        logAndAbort(ErrCurrentDirectory, err)
    }

    directory = filepath.Base(working)
    if directory == "." || directory == string(filepath.Separator) {
        logAndAbort(ErrCurrentDirectory, "Invalid working directory path")
    }

    bytes, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        logAndAbort(ErrWorkingBranch, err)
    }

    branch = strings.TrimSpace(string(bytes))
}

type MatchKind int

const (
    KindDir MatchKind = iota
    KindBra
)

func (this *MatchKind) IsDirMatch(target string) bool {
    return *this == KindDir && strings.EqualFold(directory, target)
}

func (this *MatchKind) IsBraMatch(target string) bool {
    return *this == KindBra && strings.EqualFold(branch, target)
}

func runCommand(wg *sync.WaitGroup, idx int, command string) {
    if wg != nil {
        defer wg.Done()
    }

    fmt.Printf("\x1b[90m+ %s\x1b[0m\n", command)
    child := exec.Command("sh", "-c", command)

    stdout, err := child.StdoutPipe()
    if err != nil {
        logAndAbort(ErrRunningCommand, command, err)
    }

    // Merge streams. Not only for error reporting but some info and debug logs
    // are also within the STDERR stream e.g. docker-compose.
    child.Stderr = child.Stdout
    if err := child.Start(); err != nil {
        logAndAbort(ErrRunningCommand, command, err)
    }

    reader := bufio.NewReader(stdout)
    colour := colours[idx%len(colours)]

    for {
        line, err := reader.ReadString('\n')

        if err == io.EOF {
            break
        } else if err != nil {
            logAndAbort(ErrRunningCommand, command, err)
        }

        fmt.Printf("\x1b[1;%sm%d\x1b[0m %s", colour, idx, line)
    }

    if err := child.Wait(); err != nil {
        logAndAbort(ErrRunningCommand, command, err)
    }

    fmt.Printf("\x1b[90m- %s\x1b[0m\n", command)
}

type EnvironmentEntry struct {
    // Trigger for where/when the command(s) run.
    kind MatchKind
    // Remaining tail segments.
    target string
    // Optional flag for running commands concurrently.
    isAsync bool
    // Shell commands split by a delimiter.
    commands []string
}

func (this *EnvironmentEntry) FromKeyValue(key, value string) {
    buffer := key[4:]
    if strings.HasPrefix(buffer, "ASYNC_") {
        this.isAsync = true
        buffer = buffer[6:]
    }

    switch {
    case strings.HasPrefix(buffer, "DIR_"):
        this.kind = KindDir
    case strings.HasPrefix(buffer, "BRA_"):
        this.kind = KindBra
    default:
        logAndAbort(ErrInvalidEntry, key)
    }

    buffer = buffer[4:]
    if len(buffer) == 0 {
        logAndAbort(ErrInvalidEntry, key)
    }

    parts := strings.Split(value, ",")
    for idx := 0; idx < len(parts); idx++ {
        command := parts[idx]
        parts[idx] = strings.TrimSpace(command)
    }

    this.target = buffer
    this.commands = parts
}

func (this *EnvironmentEntry) CanRun() bool {
    return this.kind.IsDirMatch(this.target) || this.kind.IsBraMatch(this.target)
}

func (this *EnvironmentEntry) StartAsync() {
    var wg sync.WaitGroup

    commandCount := len(this.commands)
    wg.Add(commandCount)

    for idx := 0; idx < commandCount; idx++ {
        command := this.commands[idx]
        go runCommand(&wg, idx, command)
    }

    wg.Wait()
}

func (this *EnvironmentEntry) Start() {
    for idx := 0; idx < len(this.commands); idx++ {
        command := this.commands[idx]
        runCommand(nil, idx, command)
    }
}

func parseEnvVariable(variable string) (EnvironmentEntry, bool) {
    var entry EnvironmentEntry

    if len(variable) <= 4 || variable[:4] != "EVC_" {
        return entry, false
    }

    key, value, found := strings.Cut(variable, "=")
    // NOTE: Standard library always returns at least one equal sign in the
    // environment variable list so the check is redundant.
    if !found || len(value) == 0 {
        return entry, false
    }

    entry.FromKeyValue(key, value)
    return entry, true
}

func main() {
    envVariables := os.Environ()

    for idx := 0; idx < len(envVariables); idx++ {
        variable := envVariables[idx]

        entry, isValid := parseEnvVariable(variable)
        if !isValid || !entry.CanRun() {
            continue
        }

        if entry.isAsync {
            entry.StartAsync()
        } else {
            entry.Start()
        }
    }

    fmt.Printf("\nenvcmd@%s\n", version)
}