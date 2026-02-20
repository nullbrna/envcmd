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
    colours   = []string{"94", "95", "96"} // Blue, Magenta, Cyan
    directory string
    branch    string
)

func errLog(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "\x1b[1;31mERROR\x1b[0m "+format+"\n", args...)
}

func runCommand(wg *sync.WaitGroup, idx int, cmd string) {
    defer wg.Done()

    fmt.Printf("\x1b[90m+ %s\x1b[0m\n", cmd)
    child := exec.Command("sh", "-c", cmd)

    stdout, err := child.StdoutPipe()
    if err != nil {
        errLog("reading stdout: %v", err)
        return
    }

    // Merge both streams, some applications use STDERR for non-error related
    // logs e.g docker-compose and needed to avoid swallowing errors.
    child.Stderr = child.Stdout

    if err := child.Start(); err != nil {
        errLog("unable to start: %v", err)
        return
    }

    reader := bufio.NewScanner(stdout)
    for reader.Scan() {
        output := reader.Text()
        // Rotate per-command through ANSI colour codes. Particularly ideal for
        // running concurrently, as streams are logging simultaneously.
        colour := colours[idx%len(colours)]

        fmt.Printf("\x1b[1;%sm%d\x1b[0m %s\n", colour, idx, output)
    }

    if err := reader.Err(); err != nil {
        errLog("reading output: %v", err)
    }

    if err := child.Wait(); err != nil {
        errLog("awaiting completion: %v", err)
        return
    }

    fmt.Printf("\x1b[90m- %s\x1b[0m\n", cmd)
}

type Kind int

const (
    dir Kind = iota
    bra
)

type Entry struct {
    async    bool
    kind     Kind
    target   string
    commands []string
}

func (this *Entry) CanRun() bool {
    // NOTE: Case-insensitive comparison.
    return (this.kind == dir && strings.EqualFold(directory, this.target)) ||
        (this.kind == bra && strings.EqualFold(branch, this.target))
}

func (this *Entry) Start() {
    // NOTE: Useless for synchronous branch but code is a little simpler with a
    // negligible performance hit.
    var wg sync.WaitGroup
    count := len(this.commands)

    wg.Add(count)
    for idx := 0; idx < count; idx++ {
        cmd := this.commands[idx]

        // NOTE: Similar to the above, this check is unneeded every iteration
        // but it's an insignificant hit.
        if this.async {
            go runCommand(&wg, idx, cmd)
        } else {
            runCommand(&wg, idx, cmd)
        }
    }

    wg.Wait()
}

func (this *Entry) Build(key, val string) error {
    buffer := key
    if strings.HasPrefix(buffer, "ASYNC_") {
        this.async = true
        buffer = buffer[6:]
    }

    switch {
    case strings.HasPrefix(buffer, "DIR_"):
        this.kind = dir
    case strings.HasPrefix(buffer, "BRA_"):
        this.kind = bra
    default:
        return fmt.Errorf("invalid kind for %s", key)
    }

    // Strip the matched prefix above, leaving the rest untouched. No need to
    // validate the format of the following as it's completely user-defined.
    buffer = buffer[4:]
    if len(buffer) == 0 {
        return fmt.Errorf("invalid target for %s", key)
    }

    // NOTE: Nothing is trimmed, so whitespace is preserved.
    this.target, this.commands = buffer, strings.Split(val, ",")
    return nil
}

func init() {
    path, err := os.Getwd()
    if err != nil {
        errLog("reading directory: %v", err)
    }

    output, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        errLog("reading branch: %v", err)
    }

    // NOTE: If either of the above fail, default strings are assigned.
    directory, branch = filepath.Base(path), strings.TrimSpace(string(output))
}

func main() {
    fmt.Printf("envcmd@%s\n", version)

    // Somehow unable to determine the working directory & branch name so
    // there's no point going any further.
    if directory == "" && branch == "" {
        return
    }

    vars := os.Environ()
    for idx := 0; idx < len(vars); idx++ {
        // Handle cases where commands can use an equals sign i.e. setting
        // inline environment variables by only getting the first split.
        pair := strings.SplitN(vars[idx], "=", 2)

        key, val := pair[0], pair[1]
        if !strings.HasPrefix(key, "EVC_") {
            continue
        }

        var entry Entry
        if err := entry.Build(key[4:], val); err != nil {
            errLog("parsing env: %v", err)
            continue
        } else if entry.CanRun() {
            entry.Start()
        }
    }
}