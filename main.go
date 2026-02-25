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
    colours []string
    // Name of the working directory.
    directory string
    // Current branch resolved via spawned process.
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

type KindOfMatch int

const (
    // Trigger on working directory match.
    kindDir KindOfMatch = iota
    // Trigger on current branch match.
    kindBra
)

func (this *KindOfMatch) IsDirMatch(target string) bool {
    return *this == kindDir && strings.EqualFold(directory, target)
}

func (this *KindOfMatch) IsBraMatch(target string) bool {
    return *this == kindBra && strings.EqualFold(branch, target)
}

type EnvironmentEntry struct {
    // Trigger for where/when the command(s) run.
    kind KindOfMatch
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

    wg.Wait()
}

func runCommand(wg *sync.WaitGroup, index int, command string) {
    if wg != nil {
        defer wg.Done()
    }

    fmt.Printf("\x1b[90m+ %s\x1b[0m\n", command)
    child := exec.Command("sh", "-c", command)

    stdout, err := child.StdoutPipe()
    if err != nil {
        logError("connecting to '%s' output: %v", command, err)
        return
    }

    // NOTE: Merge streams, not only for error reporting but some info/debug
    // logs are also within the STDERR stream e.g. docker-compose.
    child.Stderr = child.Stdout
    if err := child.Start(); err != nil {
        logError("unable to start '%s': %v", command, err)
        return
    }

    reader := bufio.NewReader(stdout)
    for {
        line, err := reader.ReadString('\n')

        if err == io.EOF {
            break
        } else if err != nil {
            logError("reading from '%s': %v", command, err)
            continue
        }

        colour := colours[index%len(colours)]
        fmt.Printf("\x1b[1;%sm%d\x1b[0m %s", colour, index, line)
    }

    if err := child.Wait(); err != nil {
        logError("awaiting completion for '%s': %v", command, err)
        return
    }

    fmt.Printf("\x1b[90m- %s\x1b[0m\n", command)
}

func parseAndRun(env string) {
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
        logError("invalid kind for '%s'", key)
        return
    }

    buffer = buffer[4:]
    if len(buffer) == 0 {
        logError("invalid target for '%s'", key)
        return
    }

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