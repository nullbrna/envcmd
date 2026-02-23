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

var version = "v0.0.0"

var (
    commandColours []string
    directoryName  string
    branchName     string
)

func logError(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "\x1b[1;31mERROR\x1b[0m "+format+"\n", args...)
}

func init() {
    // Bright Blue, Bright Magenta, Bright Cyan
    commandColours = []string{"94", "95", "96"}

    absoluteDirPath, err := os.Getwd()
    if err != nil {
        logError("reading directory: %v", err)
    }

    directoryName = filepath.Base(absoluteDirPath)

    branchOutput, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        logError("reading branch (may not be within a repository): %v", err)
    }

    branchName = strings.TrimSpace(string(branchOutput))
}

type KindOfMatch int

const (
    kindDir KindOfMatch = iota
    kindBra
)

func (this *KindOfMatch) IsDirMatch(target string) bool {
    return *this == kindDir && strings.EqualFold(directoryName, target)
}

func (this *KindOfMatch) IsBraMatch(target string) bool {
    return *this == kindBra && strings.EqualFold(branchName, target)
}

type EnvironmentEntry struct {
    kind     KindOfMatch
    target   string
    isAsync  bool
    commands []string
}

func (this *EnvironmentEntry) BuildNew(key, value string) error {
    buffer := key
    if strings.HasPrefix(buffer, "ASYNC_") {
        this.isAsync = true
        buffer = buffer[6:]
    }

    switch {
    case strings.HasPrefix(buffer, "DIR_"):
        this.kind = kindDir
    case strings.HasPrefix(buffer, "BRA_"):
        this.kind = kindBra
    default:
        return fmt.Errorf("invalid kind for %s", key)
    }

    buffer = buffer[4:]
    if len(buffer) == 0 {
        return fmt.Errorf("invalid target for %s", key)
    }

    this.target, this.commands = buffer, strings.Split(value, ",")
    return nil
}

func (this *EnvironmentEntry) CanRun() bool {
    return this.kind.IsDirMatch(this.target) || this.kind.IsBraMatch(this.target)
}

func (this *EnvironmentEntry) StartCommands() {
    var wg sync.WaitGroup
    commandCount := len(this.commands)

    wg.Add(commandCount)
    for i := 0; i < commandCount; i++ {
        if this.isAsync {
            go this.run(&wg, i, this.commands[i])
        } else {
            this.run(&wg, i, this.commands[i])
        }
    }

    wg.Wait()
}

func (this *EnvironmentEntry) run(wg *sync.WaitGroup, index int, command string) {
    defer wg.Done()

    fmt.Printf("\x1b[90m+ %s\x1b[0m\n", command)
    child := exec.Command("sh", "-c", command)

    stdout, err := child.StdoutPipe()
    if err != nil {
        logError("reading stdout: %v", err)
        return
    }

    child.Stderr = child.Stdout

    if err := child.Start(); err != nil {
        logError("unable to start %s: %v", command, err)
        return
    }

    reader := bufio.NewScanner(stdout)
    for reader.Scan() {
        output, colour := reader.Text(), index%len(commandColours)
        fmt.Printf("\x1b[1;%sm%d\x1b[0m %s\n", commandColours[colour], index, output)
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

func main() {
    fmt.Printf("envcmd@%s\n", version)

    if directoryName == "" && branchName == "" {
        os.Exit(1)
    }

    environmentVars := os.Environ()
    for i := 0; i < len(environmentVars); i++ {
        keyAndValue := strings.SplitN(environmentVars[i], "=", 2)

        key, value := keyAndValue[0], keyAndValue[1]
        if strings.HasPrefix(key, "EVC_") {
            key = key[4:]
        } else {
            continue
        }

        var entry EnvironmentEntry
        if err := entry.BuildNew(key, value); err != nil {
            logError("parsing %s: %v", key, err)
            continue
        } else if entry.CanRun() {
            entry.StartCommands()
        }
    }
}