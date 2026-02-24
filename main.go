package main

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "sync"
)

var version = "v0.0.0" // Program version passed at build-time.

var (
    // ANSI colour codes rotated through for each running command. Particularly
    // useful for concurrent output.
    colours []string
    // Base name of the working directory.
    directory string
    // Current Git branch resolved via spawned process.
    branch string
)

func logError(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "\x1b[1;31mERROR\x1b[0m "+format+"\n", args...)
}

func init() {
    colours = []string{"94", "95", "96"} // Blue, Magenta, Cyan

    absolutePath, err := os.Getwd()
    if err != nil {
        logError("reading directory: %v", err)
        os.Exit(1)
    }

    directory = filepath.Base(absolutePath)

    out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        logError("reading branch (may not be within a repository): %v", err)
        os.Exit(1)
    }

    branch = strings.TrimSpace(string(out))
}

// Trigger set within the environment variable key.
// - DIR: Match working directory.
// - BRA: Match current branch.
type KindOfMatch int

const (
    kindDir KindOfMatch = iota
    kindBra
)

func (this *KindOfMatch) IsDirMatch(target string) bool {
    return *this == kindDir && strings.EqualFold(directory, target)
}

func (this *KindOfMatch) IsBraMatch(target string) bool {
    return *this == kindBra && strings.EqualFold(branch, target)
}

type EnvironmentEntry struct {
    // Typed trigger set within the key.
    kind KindOfMatch
    // Remaining tail of the key.
    target string
    // Optional flag set within the key.
    isAsync bool
    // Shell commands split by a delimiter in the value.
    commands []string
}

func (this *EnvironmentEntry) CanRun() bool {
    return this.kind.IsDirMatch(this.target) || this.kind.IsBraMatch(this.target)
}

func (this *EnvironmentEntry) Start() {
    commandCount := len(this.commands)

    if this.isAsync {
        this.spawnAndWait(commandCount)
        return
    }

    for i := 0; i < commandCount; i++ {
        command := this.commands[i]
        runCommand(nil, i, command)
    }
}

func (this *EnvironmentEntry) spawnAndWait(count int) {
    var wg sync.WaitGroup
    wg.Add(count)

    for i := 0; i < count; i++ {
        command := this.commands[i]
        go runCommand(&wg, i, command)
    }

    wg.Wait() // Block until all routines have finished.
}

func runCommand(wg *sync.WaitGroup, index int, command string) {
    if wg != nil {
        defer wg.Done() // Signal completion when running asynchronously.
    }

    fmt.Printf("\x1b[90m+ %s\x1b[0m\n", command)
    child := exec.Command("sh", "-c", command)

    stdout, err := child.StdoutPipe()
    if err != nil {
        logError("reading stdout: %v", err)
        return
    }

    child.Stderr = child.Stdout // Merge STDERR with STDOUT to capture both.
    if err := child.Start(); err != nil {
        logError("unable to start %s: %v", command, err)
        return
    }

    reader := bufio.NewScanner(stdout)
    for reader.Scan() {
        // Use the index of the command declared in the environment variable to
        // rotate through the ANSI colour codes.
        colour := colours[index%len(colours)]
        fmt.Printf("\x1b[1;%sm%d\x1b[0m %s\n", colour, index, reader.Text())
    }

    if err := reader.Err(); err != nil {
        logError("reading output from %s: %v", command, err)
    }

    if err := child.Wait(); err != nil {
        logError("awaiting completion for %s: %v", command, err)
        return
    }

    fmt.Printf("\x1b[90m- %s\x1b[0m\n", command)
}

func parseAndRun(env string) {
    // Must match: EVC_[ASYNC_]<DIR|BRA>_<TARGET>
    key, value, found := strings.Cut(env, "=")
    if !found || !strings.HasPrefix(key, "EVC_") {
        return
    }

    var entry EnvironmentEntry

    buffer := key[4:]
    if strings.HasPrefix(buffer, "ASYNC_") {
        entry.isAsync = true
        buffer = buffer[6:]
    }

    switch {
    case strings.HasPrefix(buffer, "DIR_"):
        entry.kind = kindDir
    case strings.HasPrefix(buffer, "BRA_"):
        entry.kind = kindBra
    default:
        logError("invalid kind for %s", key)
        return
    }

    buffer = buffer[4:]
    if len(buffer) == 0 {
        logError("invalid target for %s", key)
        return
    }

    // Remaining key segments need no validation.
    // NOTE: Commands aren't trimmed so whitespace is preserved.
    entry.target, entry.commands = buffer, strings.Split(value, ",")
    if entry.CanRun() {
        entry.Start()
    }
}

func main() {
    variables := os.Environ()
    for i := 0; i < len(variables); i++ {
        env := variables[i]
        parseAndRun(env)
    }

    fmt.Printf("\nenvcmd@%s\n", version)
}