package main

import "core:bufio"
import "core:fmt"
import "core:mem"
import "core:os"
import "core:path/filepath"
import "core:strings"

VERSION :: #config(VERSION, "v0.0.0")

CONTEXT: Context

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
	directory, branch:    string,
	tar_delim, cmd_delim: string,
}

load_context :: proc() {
	working_dir, wd_err := os.getwd(context.allocator)
	if wd_err != nil do abort("Failed to read working directory: %v", wd_err)

	CONTEXT.directory = strings.clone(filepath.base(working_dir), context.allocator)
	delete(working_dir)

	proc_opts: os.Process_Desc
	proc_opts.command = {"git", "branch", "--show-current"}

	proc_state, stdout, stderr, proc_err := os.process_exec(proc_opts, context.allocator)
	defer delete(stdout)
	defer delete(stderr)

	// Provide a fallback for when the user is NOT within a repository. Avoids a
	// potential early termination of directory-context commands.
	if proc_err != nil || len(stderr) > 0 || !proc_state.success {
		warn("No Git branch found")
		CONTEXT.branch = strings.clone("", context.allocator)
	} else {
		fmt_output := strings.trim_space(string(stdout))
		CONTEXT.branch = strings.clone(fmt_output, context.allocator)
	}

	CONTEXT.tar_delim = os.get_env("EVC_TAR_SEP", context.allocator)
	if len(CONTEXT.tar_delim) == 0 {
		delete(CONTEXT.tar_delim)
		CONTEXT.tar_delim = strings.clone("_", context.allocator)
	}

	CONTEXT.cmd_delim = os.get_env("EVC_CMD_SEP", context.allocator)
	if len(CONTEXT.cmd_delim) == 0 {
		delete(CONTEXT.cmd_delim)
		CONTEXT.cmd_delim = strings.clone("|||", context.allocator)
	}
}

free_context :: proc(this: ^Context) {
	delete(this.directory)
	delete(this.branch)
	delete(this.tar_delim)
	delete(this.cmd_delim)

	this^ = {}
}

run_command :: proc(idx: int, cmd: string) {
	fmt.printfln("\x1b[90m→\x1b[22m %s\x1b[0m", cmd)

	reader, writer, sys_err := os.pipe()
	if sys_err != nil do abort("Failed to pipe %v: %v", cmd, sys_err)

	proc_opts: os.Process_Desc
	proc_opts.command = {"sh", "-c", cmd}
	proc_opts.stdout = writer
	proc_opts.stderr = writer

	process, proc_err := os.process_start(proc_opts)
	if proc_err != nil {
		os.close(reader)
		os.close(writer)
		abort("Failed to start %q: %v", cmd, proc_err)
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
			cmd_col := colours[idx % len(colours)]
			fmt.printf("[\x1b[%dm%d\x1b[0m] %s", cmd_col, idx, line)
		}

		if read_err == .EOF do break
		// NOTE: LLDB can inconsistently interrupt the read.
		when ODIN_DEBUG do if read_err == .Unknown do continue

		if read_err != nil {
			os.close(reader)
			abort("Failed to read output from %q: %v", cmd, read_err)
		}
	}

	bufio.reader_destroy(&buffer)
	os.close(reader)

	proc_state, wait_err := os.process_wait(process)
	if wait_err != nil do abort("Failed to complete %q: %v", cmd, wait_err)
	if !proc_state.success do abort("Non-zero exit code returned from %q", cmd)

	fmt.printfln("\x1b[90m←\x1b[22m %s\x1b[0m", cmd)
}

parse_and_start :: proc(env: string) {
	if !strings.has_prefix(env, "EVC_") do return

	key, assignment, value := strings.partition(env[4:], "=")
	kind, separator, target := strings.partition(key, "_")

	switch {
	case kind == "TAR" || kind == "CMD":
		return
	case kind != "DIR" && kind != "BRA":
		warn("Unexpected context kind in %q", env)
		return
	case len(target) == 0:
		warn("Missing context target in %q", env)
		return
	case separator != "_" || assignment != "=":
		warn("Unexpected key format in %q", env)
		return
	}

	// NOTE: Replaced allocations (if even made) are always freed at the end of
	// scope. The 2nd return value can be safely ignored.
	cfg, _ := strings.replace_all(target, "_", CONTEXT.tar_delim, context.temp_allocator)
	hyphenated, _ := strings.replace_all(target, "_", "-", context.temp_allocator)

	within_ctx := false
	for var in ([2]string{cfg, hyphenated}) {
		if kind == "DIR" && strings.equal_fold(var, CONTEXT.directory) do within_ctx = true
		if kind == "BRA" && strings.equal_fold(var, CONTEXT.branch) do within_ctx = true
	}

	cmd_idx := 0
	for cmd in strings.split_iterator(&value, CONTEXT.cmd_delim) do if within_ctx {
		fmt_cmd := strings.trim_space(cmd)
		if len(fmt_cmd) == 0 do continue

		run_command(cmd_idx, fmt_cmd)
		cmd_idx += 1
	}

	free_all(context.temp_allocator)
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

	load_context()
	defer free_context(&CONTEXT)

	env_vars, env_err := os.environ(context.allocator)
	defer {
		for env in env_vars do delete(env)
		delete(env_vars)
	}

	if env_err != nil do abort("Failed to read environment: %v", env_err)
	for env in env_vars do parse_and_start(env)

	fmt.printfln("\nenvcmd@%s", VERSION)
}
