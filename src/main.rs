use std::env::current_dir;
use std::env::vars;
use std::io::BufRead;
use std::io::BufReader;
use std::path::PathBuf;
use std::process::Command;
use std::process::Stdio;

const COLOUR: [&str; 3] = [
  "\x1b[94m", // Bright Blue
  "\x1b[95m", // Bright Magenta
  "\x1b[96m", // Bright Cyan
];

macro_rules! log {
  (INFO, $($arg:tt)*) => {{
    println!("\x1b[32mI.\x1b[0m {}", format_args!($($arg)*));
  }};
  (ERROR, $($arg:tt)*) => {{
    eprintln!("\x1b[31mE.\x1b[0m {}", format_args!($($arg)*));
  }};
  (COLOUR, $idx:expr, $($arg:tt)*) => {{
    let colour = COLOUR[$idx % COLOUR.len()];
    println!("{}{}.\x1b[0m {}", colour, $idx, format_args!($($arg)*));
  }};
}

#[derive(PartialEq)]
enum Kind {
  Directory,
  Branch,
}

fn kind_from_str(source: &str) -> Option<Kind> {
  if source.eq_ignore_ascii_case("dir") {
    return Some(Kind::Directory);
  } else if source.eq_ignore_ascii_case("bra") {
    return Some(Kind::Branch);
  }

  None
}

fn run_command(idx: usize, command: &str) {
  let cmd = format!("{command} 2>&1");
  let pipeout = Stdio::piped();

  log!(COLOUR, idx, "[+] {command}");

  let Ok(mut child) = Command::new("sh")
    .arg("-c")
    .arg(&cmd)
    .stdout(pipeout)
    .spawn()
  else {
    log!(ERROR, "[{idx}] unable to start");
    return;
  };

  let Some(reader) = child.stdout.take().map(BufReader::new) else {
    log!(ERROR, "[{idx}] reading stdout");
    return;
  };

  for line in reader.lines().map_while(Result::ok) {
    log!(COLOUR, idx, "{line}");
  }

  if let Err(err) = child.wait() {
    log!(ERROR, "[{idx}] awaiting completion: {err}");
    return;
  };

  log!(COLOUR, idx, "[-] {command}");
}

fn is_directory(target: &str) -> bool {
  let is_match = |path: PathBuf| {
    path
      .file_name()
      .is_some_and(|name| name.eq_ignore_ascii_case(target))
  };

  current_dir().is_ok_and(is_match)
}

fn is_branch(target: &str) -> bool {
  let Ok(child) = Command::new("git")
    .args(["rev-parse", "--abbrev-ref", "HEAD"])
    .output()
  else {
    return false;
  };

  if child.status.success() {
    return String::from_utf8_lossy(&child.stdout)
      .trim()
      .eq_ignore_ascii_case(target);
  }

  false
}

fn start(kind: Kind, target: &str, commands: Vec<&str>) {
  let matches = match kind {
    Kind::Directory => is_directory(target),
    Kind::Branch => is_branch(target),
  };

  if !matches {
    return;
  }

  log!(INFO, "[+] {target}");
  for (idx, cmd) in commands.iter().enumerate() {
    run_command(idx, cmd);
  }

  log!(INFO, "[-] {target}")
}

fn main() {
  const START: &str = "EC_";
  const SPACE: &str = "_";
  const DELIM: &str = ",";

  for (key, value) in vars() {
    let Some(key) = key.strip_prefix(START) else {
      continue;
    };

    let mut parts = key.split(SPACE);
    let commands = value.split(DELIM).collect();
    let (kind, target) = (parts.next(), parts.next());

    if let Some(Some(kind)) = kind.map(kind_from_str)
      && let Some(target) = target
    {
      start(kind, target, commands);
      continue;
    }

    log!(ERROR, "unknown format: {key}");
  }
}
