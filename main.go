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

type Kind string

const (
    dir Kind = "DIR"
    bra Kind = "BRA"
)

func (this Kind) Valid() bool {
    return this == dir || this == bra
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

func validation(segments []string, kind Kind, async bool) error {
    if async && segments[1] != "ASYNC" {
        return fmt.Errorf("expected <ASYNC> instead of %s", segments[1])
    } else if !kind.Valid() {
        return fmt.Errorf("unknown <KIND> value of %s", kind)
    }

    return nil
}

func isDirectory(target string) bool {
    dir, err := os.Getwd()
    if err != nil {
        logError("retrieving directory: %s", err)
        return false
    }

    return strings.EqualFold(filepath.Base(dir), target)
}

func isBranch(target string) bool {
    out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
    if err != nil {
        logError("retrieving branch: %s", err)
        return false
    }

    str := string(out)
    return strings.EqualFold(strings.TrimSpace(str), target)
}

func run(wg *sync.WaitGroup, idx int, command string) {
    defer wg.Done()

    logInfo("[+] %s", command)
    cmd := exec.Command("sh", "-c", command+" "+"2>&1")

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        logError("[%d] reading stdout: %v", idx, err)
        return
    }

    if err := cmd.Start(); err != nil {
        logError("[%d] unable to start: %v", idx, err)
        return
    }

    reader := bufio.NewScanner(stdout)
    for reader.Scan() {
        logColour(idx, "%s", reader.Text())
    }

    if err := cmd.Wait(); err != nil {
        logError("[%d] awaiting completion: %v", idx, err)
        return
    }

    logInfo("[-] %s", command)
}

func start(kind Kind, target string, async bool, commands []string) {
    if (kind == dir && !isDirectory(target)) || (kind == bra && !isBranch(target)) {
        return
    }

    var wg sync.WaitGroup
    for idx := 0; idx < len(commands); idx++ {
        wg.Add(1)
        cmd := commands[idx]

        if async {
            go run(&wg, idx, cmd)
            continue
        }

        run(&wg, idx, cmd)
    }

    wg.Wait()
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

        kind, target, async := Kind(segments[length-2]), segments[length-1], length == 4
        if err := validation(segments, kind, async); err != nil {
            logError("invalid key: %s", err)
            continue
        }

        commands := strings.Split(val, ",")
        start(kind, target, async, commands)
    }
}