package main

import "core:bufio"
import "core:fmt"
import "core:mem"
import "core:os"
import "core:path/filepath"
import "core:strings"

VERSION :: #config(VERSION, "v0.0.0")

CTX: Context

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
	usrdir, usrbranch: string,
	tarsep, cmdsep:    string,
}

load_context :: proc() {
	abspath, wderr := os.getwd(context.allocator)
	if wderr != nil do abort("Failed to read working directory: %v", wderr)

	CTX.usrdir = strings.clone(filepath.base(abspath), context.allocator)
	delete(abspath)

	procopts: os.Process_Desc
	procopts.command = {"git", "branch", "--show-current"}

	procstate, stdout, stderr, procerr := os.process_exec(procopts, context.allocator)
	defer delete(stdout)
	defer delete(stderr)

	// Provide a fallback for when the user is NOT within a repository. Avoids a
	// potential early termination of directory-context commands.
	if procerr != nil || len(stderr) > 0 || !procstate.success {
		warn("No Git branch found")
		CTX.usrbranch = strings.clone("", context.allocator)
	} else {
		fmtout := strings.trim_space(string(stdout))
		CTX.usrbranch = strings.clone(fmtout, context.allocator)
	}

	CTX.tarsep = os.get_env("EVC_TAR_SEP", context.allocator)
	if len(CTX.tarsep) == 0 {
		delete(CTX.tarsep)
		CTX.tarsep = strings.clone("_", context.allocator)
	}

	CTX.cmdsep = os.get_env("EVC_CMD_SEP", context.allocator)
	if len(CTX.cmdsep) == 0 {
		delete(CTX.cmdsep)
		CTX.cmdsep = strings.clone("|||", context.allocator)
	}
}

free_context :: proc(ctx: ^Context) {
	delete(ctx.usrdir)
	delete(ctx.usrbranch)
	delete(ctx.tarsep)
	delete(ctx.cmdsep)

	ctx^ = {}
}

within_context :: proc(kind, target: string) -> bool {
	fromcfg, cfgalloc := strings.replace_all(target, "_", CTX.tarsep, context.temp_allocator)
	defer if cfgalloc do delete(fromcfg)

	hyphenated, hyphalloc := strings.replace_all(target, "_", "-", context.temp_allocator)
	defer if hyphalloc do delete(hyphenated)

	variants := [2]string{fromcfg, hyphenated}
	for var in variants {
		if kind == "DIR" && strings.equal_fold(var, CTX.usrdir) do return true
		if kind == "BRA" && strings.equal_fold(var, CTX.usrbranch) do return true
	}

	return false
}

run_command :: proc(cmdidx: int, cmd: string) {
	fmt.printfln("\x1b[90m→\x1b[22m %s\x1b[0m", cmd)

	reader, writer, syserr := os.pipe()
	if syserr != nil do abort("Failed to get child process pipe: %v", syserr)

	procopts: os.Process_Desc
	procopts.command = {"sh", "-c", cmd}
	procopts.stdout = writer
	procopts.stderr = writer

	process, procerr := os.process_start(procopts)
	if procerr != nil {
		os.close(reader)
		os.close(writer)
		abort("(%s) Failed to start: %v", cmd, procerr)
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
		line, readerr := bufio.reader_read_string(&buffer, '\n', context.temp_allocator)
		if len(line) > 0 {
			cmdcol := colours[cmdidx % len(colours)]
			fmt.printf("[\x1b[%dm%d\x1b[0m] %s", cmdcol, cmdidx, line)
		}

		if readerr == .EOF do break
		if readerr != nil {
			os.close(reader)
			abort("(%s) Failed to read output: %v", cmd, readerr)
		}
	}

	bufio.reader_destroy(&buffer)
	os.close(reader)

	procstate, waiterr := os.process_wait(process)
	if waiterr != nil do abort("(%s) Failed to complete: %v", cmd, waiterr)
	if !procstate.success do abort("(%s) Non-zero exit code returned", cmd)

	fmt.printfln("\x1b[90m←\x1b[22m %s\x1b[0m", cmd)
}

parse_and_start :: proc(env: string) {
	if !strings.has_prefix(env, "EVC_") do return

	key, assignment, value := strings.partition(env[4:], "=")
	kind, separator, target := strings.partition(key, "_")

	if separator != "_" || assignment != "=" {
		warn("(%s) Unexpected key format", env)
		return
	}

	if !within_context(kind, target) do return

	cmdidx := 0
	for cmd in strings.split_iterator(&value, CTX.cmdsep) {
		fmtcmd := strings.trim_space(cmd)
		if len(fmtcmd) == 0 do continue

		run_command(cmdidx, fmtcmd)
		cmdidx += 1
	}

	free_all(context.temp_allocator)
}

main :: proc() {
	when ODIN_DEBUG {
		dbg_report_allocs :: proc(name: string, tracker: ^mem.Tracking_Allocator) {
			allocsleft := len(tracker.allocation_map)
			if allocsleft == 0 do return

			count := 1
			for allocptr in tracker.allocation_map {
				alloc := tracker.allocation_map[allocptr]

				fmt.eprintf("[\x1b[31m%s\x1b[0m] (%d/%d) ", name, count, allocsleft)
				fmt.eprintf("\x1b[33m%d\x1b[0m byte(s) - %v\n", alloc.size, alloc.location)
				count += 1
			}
		}

		genalloc: mem.Tracking_Allocator
		mem.tracking_allocator_init(&genalloc, context.allocator)
		tmpalloc: mem.Tracking_Allocator
		mem.tracking_allocator_init(&tmpalloc, context.temp_allocator)

		// NOTE: Compile time blocks don't have a "real" scope. Changes to the
		// context outlive the block, in this case the program lifetime.
		context.allocator = mem.tracking_allocator(&genalloc)
		context.temp_allocator = mem.tracking_allocator(&tmpalloc)

		defer {
			dbg_report_allocs("HEAP", &genalloc)
			mem.tracking_allocator_destroy(&genalloc)

			dbg_report_allocs("TEMP", &tmpalloc)
			mem.tracking_allocator_destroy(&tmpalloc)
		}
	}

	load_context()
	defer free_context(&CTX)

	vars, enverr := os.environ(context.allocator)
	defer {
		for env in vars do delete(env)
		delete(vars)
	}

	if enverr != nil do abort("Failed to read environment: %v", enverr)
	for env in vars do parse_and_start(env)

	fmt.printfln("\nenvcmd@%s", VERSION)
}
