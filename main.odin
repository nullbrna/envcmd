package main

import "core:bufio"
import "core:fmt"
import "core:mem"
import "core:os"
import "core:path/filepath"
import "core:strings"

VERSION :: #config(VERSION, "v0.0.0")

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

Context :: struct {
	directory, branch: string,
}

context_new :: proc() -> (this: Context) {
	working_dir, wd_err := os.getwd(context.temp_allocator)
	if wd_err == nil {
		this.directory = strings.clone(filepath.base(working_dir), context.allocator)

		// Replace common directory characters in-place that won't be present in
		// the environment variable key. To be searched with [os.get_env].
		bytes := transmute([]byte)this.directory
		for &char in bytes do if char == '-' || char == '.' do char = '_'
	} else do warn("Failed to read working directory: %v", wd_err)

	proc_opts: os.Process_Desc
	proc_opts.command = {"git", "branch", "--show-current"}
	proc_state, stdout, stderr, proc_err := os.process_exec(proc_opts, context.temp_allocator)

	if proc_err == nil && len(stderr) == 0 && proc_state.success {
		this.branch = strings.clone(strings.trim_space(string(stdout)), context.allocator)

		// Replace common branch characters in-place that won't be present in
		// the environment variable key. To be searched with [os.get_env].
		bytes := transmute([]byte)this.branch
		for &char in bytes do if char == '-' || char == '/' do char = '_'
	} else do warn("No Git branch found")

	free_all(context.temp_allocator)
	return
}

context_env_values :: proc(this: ^Context) -> (values: [3]string) {
	ALLOCATOR := context.temp_allocator

	// NOTE: [Context] values can default to empty strings. Although by design,
	// a corresponding empty key tail can subsequently match and run. To avoid
	// this, the values are length checked.
	upper_dir := strings.to_upper(this.directory, ALLOCATOR)
	upper_branch := strings.to_upper(this.branch, ALLOCATOR)

	if len(this.directory) > 0 {
		key := fmt.aprintf("EVC_DIR_%s", upper_dir, allocator = ALLOCATOR)
		values[0] = os.get_env(key, context.allocator)
	}

	if len(this.branch) > 0 {
		key := fmt.aprintf("EVC_BRA_%s", upper_branch, allocator = ALLOCATOR)
		values[1] = os.get_env(key, context.allocator)
	}

	if len(this.directory) > 0 && len(this.branch) > 0 {
		key := fmt.aprintf("EVC_ALL_%s__%s", upper_dir, upper_branch, allocator = ALLOCATOR)
		values[2] = os.get_env(key, context.allocator)
	}

	free_all(ALLOCATOR)
	return
}

context_free :: proc(this: ^Context) {
	delete(this.directory)
	delete(this.branch)

	this^ = {}
}

run_command :: proc(idx: int, command: string) {
	fmt.printfln("\x1b[90m→\x1b[22m %s\x1b[0m", command)

	reader, writer, sys_err := os.pipe()
	if sys_err != nil do abort("Failed to pipe %q: %v", command, sys_err)

	proc_opts: os.Process_Desc
	proc_opts.command = {"sh", "-c", command}
	proc_opts.stdout = writer
	proc_opts.stderr = writer

	process, proc_err := os.process_start(proc_opts)
	if proc_err != nil {
		os.close(reader)
		os.close(writer)
		abort("Failed to start %q: %v", command, proc_err)
	}

	// Documentation from [os.pipe] - "When a parent passes one of the ends of
	// the pipe to the child process, that end of the pipe needs to be closed by
	// the parent, before any data is attempted to be read."
	os.close(writer)

	// Green, Yellow, Blue, Magenta, and Cyan ANSI codes.
	@(static) colours := [5]u8{32, 33, 34, 35, 36}

	buffer: bufio.Reader
	bufio.reader_init(&buffer, os.to_stream(reader), 4096, context.temp_allocator)

	for {
		line, read_err := bufio.reader_read_string(&buffer, '\n', context.temp_allocator)
		if len(line) > 0 {
			colour := colours[idx % len(colours)]
			fmt.printf("[\x1b[%dm%d\x1b[0m] %s", colour, idx, line)
		}

		if read_err == .EOF do break
		// NOTE: LLDB can inconsistently interrupt the read.
		when ODIN_DEBUG do if read_err == .Unknown do continue

		if read_err != nil {
			os.close(reader)
			abort("Failed to read output from %q: %v", command, read_err)
		}
	}

	bufio.reader_destroy(&buffer)
	os.close(reader)

	proc_state, wait_err := os.process_wait(process)
	if wait_err != nil do abort("Failed to complete %q: %v", command, wait_err)
	if !proc_state.success do abort("Non-zero exit code returned from %q", command)

	fmt.printfln("\x1b[90m←\x1b[22m %s\x1b[0m", command)
}


main :: proc() {
	when ODIN_DEBUG {
		dbg_report_allocs :: proc(key: string, track_alloc: ^mem.Tracking_Allocator) {
			dangling_count := len(track_alloc.allocation_map)
			if dangling_count == 0 do return

			count := 1
			for alloc_ptr in track_alloc.allocation_map {
				alloc := track_alloc.allocation_map[alloc_ptr]

				fmt.eprintf("[\x1b[31m%s\x1b[0m] (%d/%d) ", key, count, dangling_count)
				fmt.eprintf("\x1b[33m%d\x1b[0m byte(s) - %v\n", alloc.size, alloc.location)
				count += 1
			}
		}

		gen_allocator: mem.Tracking_Allocator
		mem.tracking_allocator_init(&gen_allocator, context.allocator)
		tmp_allocator: mem.Tracking_Allocator
		mem.tracking_allocator_init(&tmp_allocator, context.temp_allocator)

		// NOTE: Compile time blocks don't have a "real" scope. Changes to the
		// context outlive the block, in this case the program lifetime.
		context.allocator = mem.tracking_allocator(&gen_allocator)
		context.temp_allocator = mem.tracking_allocator(&tmp_allocator)

		defer {
			dbg_report_allocs("HEAP", &gen_allocator)
			mem.tracking_allocator_destroy(&gen_allocator)

			dbg_report_allocs("TEMP", &tmp_allocator)
			mem.tracking_allocator_destroy(&tmp_allocator)
		}
	}

	ctx := context_new()
	defer context_free(&ctx)

	for value in context_env_values(&ctx) do if len(value) > 0 {
		command_idx := 0
		header_copy := value

		// NOTE: Use a cheap header copy. Passing [value] directly would mutate
		// the owned string and leave an invalid header.
		for command in strings.split_iterator(&header_copy, "|||") {
			trimmed := strings.trim_space(command)
			if len(trimmed) == 0 do continue

			run_command(command_idx, trimmed)
			command_idx += 1
		}

		free_all(context.temp_allocator)
		delete(value)
	}

	fmt.printfln("\nenvcmd@%s", VERSION)
}
