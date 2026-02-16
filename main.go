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

var colours = []string{
    "\x1b[94m", // Bright Blue
    "\x1b[95m", // Bright Magenta
    "\x1b[96m", // Bright Cyan
}

func logInfo(format string, args ...any) {
    text := fmt.Sprintf(format, args...)
    fmt.Printf("[\x1b[32mI\x1b[0m] %s\n", text)
}

func logError(format string, args ...any) {
    text := fmt.Sprintf(format, args...)
    fmt.Fprintf(os.Stderr, "[\x1b[31mE\x1b[0m] %s\n", text)
}

func logColour(idx int, format string, args ...any) {
    text := fmt.Sprintf(format, args...)
    fmt.Printf("[%s%d\x1b[0m] %s\n", colours[idx%len(colours)], idx, text)
}

type Kind string

const (
    dir Kind = "DIR"
    bra Kind = "BRA"
)

func (this Kind) Valid() bool {
    return this == dir || this == bra
}

func (this Kind) isDirectory(target string) bool {
    if this != dir {
        return false
    }

    dir, err := os.Getwd()
    if err != nil {
        logError("retrieving directory: %s", err)
        return false
    }

    return strings.EqualFold(filepath.Base(dir), target)
}

func (this Kind) isBranch(target string) bool {
    if this != bra {
        return false
    }

    out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        logError("retrieving branch: %s", err)
        return false
    }

    str := string(out)
    return strings.EqualFold(strings.TrimSpace(str), target)
}

type Entry struct {
    async    bool
    kind     Kind
    target   string
    commands []string
}

func (this *Entry) Validate(segments []string) error {
    if this.async && segments[1] != "ASYNC" {
        return fmt.Errorf("expected <ASYNC> instead of %s", segments[1])
    } else if !this.kind.Valid() {
        return fmt.Errorf("unknown <KIND> value of %s", this.kind)
    }

    return nil
}

func (this *Entry) Start() {
    if !this.kind.isDirectory(this.target) && !this.kind.isBranch(this.target) {
        return
    }

    count := len(this.commands)
    var wg sync.WaitGroup
    wg.Add(count)

    for idx := 0; idx < count; idx++ {
        cmd := this.commands[idx]

        if this.async {
            go runCommand(&wg, idx, cmd)
        } else {
            runCommand(&wg, idx, cmd)
        }
    }

    wg.Wait()
}

func runCommand(wg *sync.WaitGroup, idx int, cmd string) {
    defer wg.Done()

    logInfo("[+] %s", cmd)
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

    logInfo("[-] %s", cmd)
}

func main() {
    envs := os.Environ()

    for idx := 0; idx < len(envs); idx++ {
        segments := strings.SplitN(envs[idx], "=", 2)

        key, val := segments[0], segments[1]
        if !strings.HasPrefix(key, "EVC_") {
            continue
        }

        segments = strings.Split(key, "_")
        length := len(segments)

        if length != 3 && length != 4 {
            logError("malformed key: %s", key)
            continue
        }

        entry := Entry{
            async:    length == 4,
            kind:     Kind(segments[length-2]),
            target:   segments[length-1],
            commands: strings.Split(val, ","),
        }

        if err := entry.Validate(segments); err != nil {
            logError("invalid key: %s", err)
            continue
        }

        entry.Start()
    }
}