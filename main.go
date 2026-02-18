package main

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
)

var version = "v0.0.0"

// Bright Blue, Bright Magenta, Bright Cyan
var colours = []string{"94m", "95m", "96m"}

func logError(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "\x1b[31mE\x1b[0m "+format+"\n", args...)
}

func logMuted(prefix, format string, args ...any) {
    fmt.Printf("\x1b[90m"+prefix+" "+format+"\x1b[0m\n", args...)
}

func logColour(idx int, format string, args ...any) {
    colour, prefix := colours[idx%len(colours)], strconv.Itoa(idx)
    fmt.Printf("\x1b["+colour+prefix+"\x1b[0m "+format+"\n", args...)
}

func isDirMatch(target string) bool {
    dir, err := os.Getwd()
    if err != nil {
        logError("reading directory: %v", err)
        return false
    }

    // NOTE: Case-insensitive comparison.
    return strings.EqualFold(filepath.Base(dir), target)
}

func isBraMatch(target string) bool {
    out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        logError("reading branch: %v", err)
        return false
    }

    str := string(out)
    // NOTE: Case-insensitive comparison.
    return strings.EqualFold(strings.TrimSpace(str), target)
}

func runCommand(wg *sync.WaitGroup, idx int, cmd string) {
    defer wg.Done()
    logMuted("+", "[%d] %s", idx, cmd)

    // Combine both STDOUT & STDERR streams into one. Some use STDERR for
    // non-error logs e.g docker-compose.
    piped := fmt.Sprintf("%s 2>&1", cmd)
    child := exec.Command("sh", "-c", piped)

    out, err := child.StdoutPipe()
    if err != nil {
        logError("[%d] reading stdout: %v", idx, err)
        return
    }

    if err := child.Start(); err != nil {
        logError("[%d] unable to start: %v", idx, err)
        return
    }

    reader := bufio.NewScanner(out)
    for reader.Scan() {
        logColour(idx, "%s", reader.Text())
    }

    if err := child.Wait(); err != nil {
        logError("[%d] awaiting completion: %v", idx, err)
        return
    }

    logMuted("-", "[%d] %s", idx, cmd)
}

type Entry struct {
    async        bool
    kind, target string
    commands     []string
}

func (this *Entry) MutBuild(key, val string) error {
    asy := "ASYNC_"
    dir := "DIR_"
    bra := "BRA_"

    unchanged := key // NOTE: Only needed (string header) for error logs.
    if strings.HasPrefix(key, asy) {
        this.async = true
        key = key[len(asy):]
    }

    switch {
    case strings.HasPrefix(key, dir):
        this.kind = "DIR"
        key = key[len(dir):]
    case strings.HasPrefix(key, bra):
        this.kind = "BRA"
        key = key[len(bra):]
    default:
        return fmt.Errorf("unrecognised/missing <KIND>/<TARGET> in %s", unchanged)
    }

    this.target = key
    this.commands = strings.Split(val, ",")
    return nil
}

func (this *Entry) Start() {
    switch {
    case this.kind == "DIR" && isDirMatch(this.target):
        break
    case this.kind == "BRA" && isBraMatch(this.target):
        break
    default:
        return
    }

    count := len(this.commands)
    // NOTE: Useless WaitGroup for synchronous branch. Code is a little simpler
    // with negligible performance hit.
    var wg sync.WaitGroup
    wg.Add(count)

    for idx := 0; idx < count; idx++ {
        cmd := this.commands[idx]

        // NOTE: Similar to the WaitGroup, this check is unneeded every
        // iteration but it's insignificant.
        if this.async {
            go runCommand(&wg, idx, cmd)
        } else {
            runCommand(&wg, idx, cmd)
        }
    }

    wg.Wait()
}

func main() {
    fmt.Printf("envcmd@%s\n", version)

    envs, prefix := os.Environ(), "EVC_"
    for idx := 0; idx < len(envs); idx++ {
        segments := strings.Split(envs[idx], "=")

        key, val := segments[0], segments[1]
        if !strings.HasPrefix(key, prefix) {
            continue
        } else {
            key = key[len(prefix):] // Skip past the prefix.
        }

        var entry Entry
        if err := entry.MutBuild(key, val); err != nil {
            logError("parsing env: %v", err)
            continue
        }

        entry.Start()
    }
}