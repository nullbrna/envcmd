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

var version = "v0.0.0" // Program version passed at build-time.

var (
    // ANSI colour codes rotated through for each running command.
    COLOURS []string
    // Name of the working directory.
    DIRECTORY string
    // Current branch resolved via spawned process.
    BRANCH string
)

func logAndAbort(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "\x1b[1;31mE\x1b[0m "+format+"\n", args...)
    os.Exit(1)
}

func init() {
    COLOURS = []string{"94", "95", "96"} // Blue, Magenta, Cyan

    absolutePath, err := os.Getwd()
    if err != nil {
        logAndAbort("reading directory: %v", err)
    }

    DIRECTORY = filepath.Base(absolutePath)

    out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        logAndAbort("reading branch (may not be within a repository): %v", err)
    }

    BRANCH = strings.TrimSpace(string(out))
}

type MatchKind int

const (
    KindDir MatchKind = iota
    KindBra
)

func (this *MatchKind) IsDirMatch(target string) bool {
    return *this == KindDir && strings.EqualFold(DIRECTORY, target)
}

func (this *MatchKind) IsBraMatch(target string) bool {
    return *this == KindBra && strings.EqualFold(BRANCH, target)
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

func (this *EnvironmentEntry) CanRun() bool {
    return this.kind.IsDirMatch(this.target) || this.kind.IsBraMatch(this.target)
}

func (this *EnvironmentEntry) Start() {
    cmdCount := len(this.commands)

    if this.isAsync {
        var wg sync.WaitGroup
        wg.Add(cmdCount)

        // Each command spawns a new routine. Immediately wait for all spawned
        // processes to finish, allows for concurrent STDOUT streams.
        for idx := 0; idx < cmdCount; idx++ {
            go runCommand(&wg, idx, this.commands[idx])
        }

        wg.Wait()
        return
    }

    for idx := 0; idx < cmdCount; idx++ {
        runCommand(nil, idx, this.commands[idx])
    }
}

func runCommand(wg *sync.WaitGroup, idx int, cmd string) {
    if wg != nil {
        defer wg.Done()
    }

    fmt.Printf("\x1b[90m+ %s\x1b[0m\n", cmd)
    child := exec.Command("sh", "-c", cmd)

    stdout, err := child.StdoutPipe()
    if err != nil {
        logAndAbort("connecting to '%s' output: %v", cmd, err)
    }

    // Merge streams, not only for error reporting but some info/debug logs are
    // also within the STDERR stream e.g. docker-compose.
    child.Stderr = child.Stdout
    if err := child.Start(); err != nil {
        logAndAbort("unable to start '%s': %v", cmd, err)
    }

    colour, reader := COLOURS[idx%len(COLOURS)], bufio.NewReader(stdout)
    for {
        line, err := reader.ReadString('\n')

        if err == io.EOF {
            break
        } else if err != nil {
            logAndAbort("reading from '%s': %v", cmd, err)
        }

        fmt.Printf("\x1b[1;%sm%d\x1b[0m %s", colour, idx, line)
    }

    // Resolve once the command completes, can error for a non-zero exit.
    if err := child.Wait(); err != nil {
        logAndAbort("aborted from '%s': %v", cmd, err)
    }

    fmt.Printf("\x1b[90m- %s\x1b[0m\n", cmd)
}

func parseAndRun(variable string) {
    key, value, found := strings.Cut(variable, "=")
    if !found || !strings.HasPrefix(key, "EVC_") {
        return
    }

    var entry EnvironmentEntry

    // 1. Skip past the prefix and check for optional concurrent flag.
    buffer := key[4:]
    if strings.HasPrefix(buffer, "ASYNC_") {
        entry.isAsync = true
        buffer = buffer[6:]
    }

    // 2. Determine the trigger that's at the 1st or 2nd position.
    switch {
    case strings.HasPrefix(buffer, "DIR_"):
        entry.kind = KindDir
    case strings.HasPrefix(buffer, "BRA_"):
        entry.kind = KindBra
    default:
        logAndAbort("invalid kind for '%s'", key)
    }

    // 3. Skip past the trigger and ensure there's tail segments following.
    buffer = buffer[4:]
    if len(buffer) == 0 {
        logAndAbort("invalid target for '%s'", key)
    }

    segments := strings.Split(value, ",")
    for idx := 0; idx < len(segments); idx++ {
        segments[idx] = strings.TrimSpace(segments[idx])
    }

    entry.target, entry.commands = buffer, segments
    if entry.CanRun() {
        entry.Start()
    }
}

func main() {
    variables := os.Environ()
    for idx := 0; idx < len(variables); idx++ {
        parseAndRun(variables[idx])
    }

    fmt.Printf("\nenvcmd@%s\n", version)
}