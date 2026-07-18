use std::ffi::OsStr;
use std::fmt::Display;
use std::io::BufRead;
use std::io::BufReader;
use std::process::Command;
use std::process::Stdio;

const VERSION: &str = env!("CARGO_PKG_VERSION");
/// Green, Yellow, Blue, Magenta, and Cyan ANSI codes respectively.
const COLOURS: [u8; 5] = [32, 33, 34, 35, 36];

/// Helper to panic with a custom error message.
/// NOTE: Generic return for unwrapping "else" branch.
fn die<T>(err: impl Display) -> T {
    eprintln!("\x1b[1;31m→\x1b[0m {err}");
    std::process::exit(1);
}

fn read_working_context() -> (String, String) {
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

fn validate_key(key: &str) -> Option<(bool, &str, &str)> {
    let (sync, rest) = match key.strip_prefix("ASYNC_") {
        Some(rest) => (false, rest),
        None => (true, key),
    };

    let (kind, target) = rest.split_once('_')?;
    if (kind != "DIR" && kind != "BRA") || target.is_empty() {
        die::<&str>("Incorrect context kind or empty target");
    }

    Some((sync, kind, target))
}

fn can_run(kind: &str, target: &str, dir: &str, branch: &str) -> bool {
    (kind == "DIR" && target.eq_ignore_ascii_case(dir))
        || (kind == "BRA" && target.eq_ignore_ascii_case(branch))
}

fn is_relevant((mut key, value): (String, String)) -> Option<(String, String)> {
    if !key.starts_with("EVC_") || value.is_empty() {
        return None;
    }

    key.drain(..4);
    Some((key, value))
}

fn launch_piped_command((idx, cmd): (usize, &str)) {
    println!("\x1b[1;90m→\x1b[22m \x1b[90m{cmd}\x1b[0m");

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

    while reader.read_line(&mut buffer).is_ok_and(|bytes| bytes > 0) {
        print!("\x1b[1;{colour}m{idx}\x1b[0m {buffer}");
        buffer.clear();
    }

    if !child.wait().unwrap_or_else(die).success() {
        die::<&str>("Non-zero exit code returned");
    }

    println!("\x1b[1;90m←\x1b[22m \x1b[90m{cmd}\x1b[0m");
}

fn parse_and_run(value: &str, sync: bool) {
    let commands = value.split("|||").map(str::trim).enumerate();

    if sync {
        commands.for_each(launch_piped_command);
        return;
    }

    std::thread::scope(|scope| {
        commands.for_each(|pair| {
            scope.spawn(move || launch_piped_command(pair));
        });
    });
}

fn main() {
    let (dir, branch) = read_working_context();

    for (key, value) in std::env::vars().filter_map(is_relevant) {
        let Some((sync, kind, target)) = validate_key(&key) else {
            continue;
        };

        if can_run(kind, target, &dir, &branch) {
            parse_and_run(&value, sync);
        }
    }

    println!("\nenvcmd@{VERSION}");
}
