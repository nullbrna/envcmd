use std::ffi::OsStr;
use std::fmt::Display;
use std::io::BufRead;
use std::io::BufReader;
use std::process::Command;
use std::process::Stdio;
use std::sync::LazyLock;

const VERSION: &str = env!("CARGO_PKG_VERSION");
/// Green, Yellow, Blue, Magenta, and Cyan ANSI codes respectively.
const COLOURS: [u8; 5] = [32, 33, 34, 35, 36];

/// 0: Working directory name.
/// 1: Current branch (empty if unavailable) name.
static CONTEXT: LazyLock<(String, String)> = LazyLock::new(load_context);

/// Helper to panic with a formatted error message.
/// NOTE: Generic return for unwrapping "else" branch.
fn die<T>(err: impl Display) -> T {
    eprintln!("\x1b[1;31m✗\x1b[0m {err}");
    std::process::exit(1);
}

fn load_context() -> (String, String) {
    let dir = std::env::current_dir()
        .unwrap_or_else(die)
        .file_name()
        .and_then(OsStr::to_str)
        .ok_or("Irregular file name suffix")
        .unwrap_or_else(die)
        .to_owned();

    let branch = Command::new("git")
        .arg("symbolic-ref")
        .arg("--short")
        .arg("HEAD")
        .output()
        .map(|out| String::from_utf8_lossy(&out.stdout).trim().to_owned())
        .unwrap_or_default();

    (dir, branch)
}

fn is_match((kind, target): (&str, &str)) -> bool {
    (kind == "DIR" && target.eq_ignore_ascii_case(&CONTEXT.0))
        || (kind == "BRA" && target.eq_ignore_ascii_case(&CONTEXT.1))
}

fn run_command((idx, cmd): (usize, &str)) {
    println!("\x1b[1;90m→\x1b[22m {cmd}\x1b[0m");

    let mut child = Command::new("sh")
        .arg("-c")
        .arg(cmd)
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit())
        .spawn()
        .unwrap_or_else(die);

    let mut reader = child
        .stdout
        .take()
        .map(BufReader::new)
        .ok_or("Missing STDOUT handle")
        .unwrap_or_else(die);

    let mut buffer = String::with_capacity(256);
    let colour = COLOURS[idx % COLOURS.len()];

    while reader.read_line(&mut buffer).is_ok_and(|n| n > 0) {
        print!("\x1b[1;{colour}m{idx}\x1b[0m {buffer}");
        buffer.clear();
    }

    if !child.wait().unwrap_or_else(die).success() {
        die::<&str>("Non-zero exit code returned");
    }

    println!("\x1b[1;90m←\x1b[22m {cmd}\x1b[0m");
}

fn parse_and_run(key: &str, value: &str) {
    let Some(key) = key.strip_prefix("EVC_") else {
        return;
    };

    let (sync, rest) = match key.strip_prefix("ASYNC_") {
        Some(rest) => (false, rest),
        None => (true, key),
    };

    let commands = value
        .split("|||")
        .map(str::trim)
        .filter(|s| s.len() > 0)
        .enumerate();

    if !rest.split_once('_').is_some_and(is_match) {
        return;
    } else if sync {
        commands.for_each(run_command);
        return;
    }

    std::thread::scope(|scope| {
        commands.for_each(|pair| {
            scope.spawn(move || run_command(pair));
        });
    });
}

fn main() {
    for (key, value) in std::env::vars() {
        parse_and_run(&key, &value);
    }

    println!("\nenvcmd@{VERSION}");
}
