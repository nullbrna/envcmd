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

var (
    version   = "v0.0.0" // Program version passed at build-time.
    colours   []string   // ANSI colour codes rotated through for each running command.
    directory string     // Name of the working directory.
    branch    string     // Current branch resolved via spawned process.
)

func logAndAbort(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "\x1b[1;31mE\x1b[0m "+format+"\n", args...)
    os.Exit(1)
}

func init() {
    // Blue, Magenta, Cyan
    colours = []string{"94", "95", "96"}

    path, err := os.Getwd()
    if err != nil {
        logAndAbort("reading directory: %v", err)
    }

    text, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        logAndAbort("reading branch (may not be within a repository): %v", err)
    }

    directory = filepath.Base(path)
    branch = strings.TrimSpace(string(text))
}

type MatchKind int

const (
    KindDir MatchKind = iota // Check for working directory name equality.
    KindBra                  // Check for current branch name equality.
)

func (this *MatchKind) IsDirMatch(target string) bool {
    return *this == KindDir && strings.EqualFold(directory, target)
}

func (this *MatchKind) IsBraMatch(target string) bool {
    return *this == KindBra && strings.EqualFold(branch, target)
}

type EnvironmentEntry struct {
    kind     MatchKind // Trigger for where/when the command(s) run.
    target   string    // Remaining tail segments.
    isAsync  bool      // Optional flag for running commands concurrently.
    commands []string  // Shell commands split by a delimiter.
}

func (this *EnvironmentEntry) FromKeyValue(key, value string) {
    // 1. Skip past the prefix and check for optional concurrent flag.
    buffer := key[4:]
    if strings.HasPrefix(buffer, "ASYNC_") {
        this.isAsync = true
        buffer = buffer[6:]
    }

    // 2. Determine the trigger that's at the 1st or 2nd position.
    switch {
    case strings.HasPrefix(buffer, "DIR_"):
        this.kind = KindDir
    case strings.HasPrefix(buffer, "BRA_"):
        this.kind = KindBra
    default:
        logAndAbort("invalid kind for '%s'", key)
    }

    // 3. Skip past the trigger and ensure there's tail segments following.
    buffer = buffer[4:]
    if len(buffer) == 0 {
        logAndAbort("invalid target for '%s'", key)
    }

    commands := strings.Split(value, ",")
    for idx := 0; idx < len(commands); idx++ {
        command := commands[idx]
        // Trim any leading and/or trailing whitespace. Primarily to cover
        // cosmetic spaces after each delimiter.
        commands[idx] = strings.TrimSpace(command)
    }

    this.target = buffer
    this.commands = commands
}

func (this *EnvironmentEntry) CanRun() bool {
    return this.kind.IsDirMatch(this.target) || this.kind.IsBraMatch(this.target)
}

func (this *EnvironmentEntry) Start() {
    count := len(this.commands)

    if this.isAsync {
        var wg sync.WaitGroup
        wg.Add(count)

        // Each command spawns a new routine. Immediately wait for all spawned
        // processes to finish, allows for concurrent STDOUT streams.
        for idx := 0; idx < count; idx++ {
            command := this.commands[idx]
            go runCommand(&wg, idx, command)
        }

        wg.Wait()
        return
    }

    for idx := 0; idx < count; idx++ {
        command := this.commands[idx]
        runCommand(nil, idx, command)
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

    colour := colours[idx%len(colours)]
    reader := bufio.NewReader(stdout)

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

func main() {
    variables := os.Environ()

    for idx := 0; idx < len(variables); idx++ {
        variable := variables[idx]

        // Take before and after the first occurrence of the assignment
        // operator. Any assignments in the commands will be respected.
        key, value, found := strings.Cut(variable, "=")
        if !found || !strings.HasPrefix(key, "EVC_") {
            continue
        }

        var entry EnvironmentEntry
        entry.FromKeyValue(key, value)

        if entry.CanRun() {
            entry.Start()
        }
    }

    fmt.Printf("\nenvcmd@%s\n", version)
}