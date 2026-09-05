package main

import "core:bufio"
import "core:fmt"
import "core:mem"
import "core:os"
import "core:path/filepath"
import "core:strings"

VERSION :: #config(VERSION, "v0.0.0")

USER_DIR, USER_BRANCH: string

abort :: proc(msg: string, args: ..any) -> ! {
	fmt.eprint("[\x1b[31mERR\x1b[0m] ")
	fmt.eprintf(msg, ..args)
	fmt.eprint("\n")
	os.exit(1)
}

warn :: proc(msg: string, args: ..any) {
	fmt.print("[\x1b[33mWAR\x1b[0m] ")
	fmt.printf(msg, ..args)
	fmt.print("\n")
}

load_context :: proc() {
	working_dir, wd_err := os.getwd(context.allocator)
	if wd_err != nil do abort("Failed to read working directory: %v", wd_err)
	else {
		USER_DIR = strings.clone(filepath.base(working_dir), context.allocator)
		delete(working_dir)
	}

	proc_opts: os.Process_Desc
	proc_opts.command = {"git", "branch", "--show-current"}

	proc_state, stdout, stderr, proc_err := os.process_exec(proc_opts, context.allocator)
	defer delete(stdout)
	defer delete(stderr)

	if proc_err != nil || len(stderr) > 0 || !proc_state.success {
		warn("No Git branch found")
		USER_BRANCH = strings.clone("", context.allocator)
		return
	}

	USER_BRANCH = strings.clone(strings.trim_space(string(stdout)), context.allocator)
}

within_context :: proc(kind, target: string) -> bool {
	hyphenated, did_alloc := strings.replace_all(target, "_", "-", context.allocator)
	defer if did_alloc do delete(hyphenated)

	variants := [2]string{target, hyphenated}
	for var in variants {
		if kind == "DIR" && strings.equal_fold(var, USER_DIR) do return true
		if kind == "BRA" && strings.equal_fold(var, USER_BRANCH) do return true
	}

	return false
}

run_command :: proc(idx: int, command: string) {
	fmt.printfln("\x1b[90m→\x1b[22m %s\x1b[0m", command)

	writer, reader, pipe_err := os.pipe()
	if pipe_err != nil do abort("Failed to get child process pipe: %v", pipe_err)

	proc_opts: os.Process_Desc
	proc_opts.command = {"sh", "-c", command}
	proc_opts.stdout = reader
	proc_opts.stderr = reader

	process, proc_err := os.process_start(proc_opts)
	if proc_err != nil {
		os.close(writer)
		os.close(reader)
		abort("(%s) Failed to start: %v", command, proc_err)
	}

	// Documentation from [os.pipe] - "When a parent passes one of the ends of
	// the pipe to the child process, that end of the pipe needs to be closed by
	// the parent, before any data is attempted to be read."
	os.close(reader)

	// Green, Yellow, Blue, Magenta, and Cyan ANSI codes.
	@(static) colours := [5]u8{32, 33, 34, 35, 36}

	buffer: bufio.Reader
	bufio.reader_init(&buffer, os.to_stream(writer), 4096, context.temp_allocator)

	for {
		line, read_err := bufio.reader_read_string(&buffer, '\n', context.temp_allocator)
		if len(line) > 0 do fmt.printf("[\x1b[%dm%d\x1b[0m] %s", colours[idx % len(colours)], idx, line)

		if read_err == .EOF do break
		else if read_err != nil {
			os.close(writer)
			abort("(%s) Failed to read output: %v", command, read_err)
		}
	}

	bufio.reader_destroy(&buffer)
	os.close(writer)

	proc_state, wait_err := os.process_wait(process)
	if wait_err != nil do abort("(%s) Failed to complete: %v", command, wait_err)
	else if !proc_state.success do abort("(%s) Non-zero exit code returned", command)

	fmt.printfln("\x1b[90m←\x1b[22m %s\x1b[0m", command)
}

parse_and_start :: proc(entry: string) {
	if !strings.has_prefix(entry, "EVC_") do return
	entry := entry[4:]

	key, assignment, value := strings.partition(entry, "=")
	if assignment != "=" do return

	kind, separator, target := strings.partition(key, "_")
	if separator != "_" do return

	if (kind != "DIR" && kind != "BRA") || len(target) == 0 {
		warn("(%s) Unexpected format", key)
		return
	} else if !within_context(kind, target) do return

	idx := 0
	for command in strings.split_iterator(&value, "|||") {
		command := strings.trim_space(command)
		if len(command) == 0 do continue

		run_command(idx, command)
		idx += 1
	}

	// NOTE: Responsible for the [run_command] allocations also.
	free_all(context.temp_allocator)
}

main :: proc() {
	when ODIN_DEBUG {
		dbg_report_allocs :: proc(name: string, tracker: ^mem.Tracking_Allocator) {
			dangling := len(tracker.allocation_map)
			if dangling == 0 do return

			count := 1
			for alloc_ptr in tracker.allocation_map {
				alloc := tracker.allocation_map[alloc_ptr]

				fmt.eprintf("[\x1b[31m%s\x1b[0m] (%d/%d) ", name, count, dangling)
				fmt.eprintf("\x1b[33m%d\x1b[0m byte(s) - %v\n", alloc.size, alloc.location)
				count += 1
			}
		}

		general_alloc: mem.Tracking_Allocator
		mem.tracking_allocator_init(&general_alloc, context.allocator)
		temp_alloc: mem.Tracking_Allocator
		mem.tracking_allocator_init(&temp_alloc, context.temp_allocator)

		// NOTE: Compile time blocks don't have a "real" scope. Changes to the
		// context outlive the block, in this case the program lifetime.
		context.allocator = mem.tracking_allocator(&general_alloc)
		context.temp_allocator = mem.tracking_allocator(&temp_alloc)

		defer {
			dbg_report_allocs("HEAP", &general_alloc)
			mem.tracking_allocator_destroy(&general_alloc)

			dbg_report_allocs("TEMP", &temp_alloc)
			mem.tracking_allocator_destroy(&temp_alloc)
		}
	}

	load_context()
	defer delete(USER_DIR)
	defer delete(USER_BRANCH)

	env_vars, env_err := os.environ(context.allocator)
	defer {
		for env in env_vars do delete(env)
		delete(env_vars)
	}

	if env_err != nil do abort("Failed to read environment: %v", env_err)
	else do for entry in env_vars do parse_and_start(entry)

	fmt.printfln("\nenvcmd@%s", VERSION)
}
