package main

import (
    "fmt"
    "os"
    "strings"
)

var COLOURS = []string{
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
    fmt.Printf("[%s%d\x1b[0m] %s\n", COLOURS[idx%len(COLOURS)], idx, text)
}

func validation(parts []string) error {
    if len(parts) == 4 && parts[1] != "ASYNC" {
        return fmt.Errorf("expected <ASYNC> instead of %s", parts[1])
    }

    var index int
    if len(parts) == 4 {
        index = 2
    } else {
        index = 1
    }

    if parts[index] != "DIR" && parts[index] != "BRA" {
        return fmt.Errorf("unknown <KIND> value of %s", parts[index])
    }

    return nil
}

func main() {
    envs := os.Environ()

    for i := 0; i < len(envs); i++ {
        parts := strings.Split(envs[i], "=")
        if len(parts) != 2 {
            continue
        }

        key, val := parts[0], parts[1]
        if !strings.HasPrefix(key, "EVC_") {
            continue
        }

        parts = strings.Split(key, "_")
        length := len(parts)

        if length != 3 && length != 4 {
            logError("malformed key: %s", key)
            continue
        }

        if err := validation(parts); err != nil {
            logError("invalid key: %s", err)
            continue
        }

        async := length == 4
        logInfo("%s = %v", key, async)

        logColour(0, "%s", val)
        logColour(1, "%s", val)
        logColour(2, "%s", val)
    }
}