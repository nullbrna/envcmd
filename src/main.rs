use std::env::current_dir;
use std::env::vars;
use std::io::BufRead;
use std::io::BufReader;
use std::path::PathBuf;
use std::process::Command;
use std::process::Stdio;
use std::thread::spawn;

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
  let output = Stdio::piped();

  log!(COLOUR, idx, "[+] {command}");

  let Ok(mut child) = Command::new("sh")
    .arg("-c")
    .arg(&cmd)
    .stdout(output)
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
  let is_name_match = |path: PathBuf| {
    let folder = path.file_name();
    folder.is_some_and(|name| name.eq_ignore_ascii_case(target))
  };

  current_dir().is_ok_and(is_name_match)
}

fn is_branch(target: &str) -> bool {
  let Ok(child) = Command::new("git")
    .arg("rev-parse")
    .arg("--abbrev-ref")
    .arg("HEAD")
    .output()
  else {
    return false;
  };

  return String::from_utf8_lossy(&child.stdout)
    .trim()
    .eq_ignore_ascii_case(target);
}

fn run_async(commands: Vec<&str>) {
  let cap = commands.len();
  let mut handles = Vec::with_capacity(cap);

  for (idx, cmd) in commands.into_iter().enumerate() {
    let cmd = cmd.to_owned();
    let handle = spawn(move || run_command(idx, &cmd));
    handles.push(handle);
  }

  for handle in handles {
    let _ = handle.join();
  }
}

fn run_sync(commands: Vec<&str>) {
  commands
    .into_iter()
    .enumerate()
    .for_each(|(idx, cmd)| run_command(idx, cmd));
}

fn start(kind: Kind, target: &str, commands: Vec<&str>, is_async: bool) {
  let is_kind_match = match kind {
    Kind::Directory => is_directory(target),
    Kind::Branch => is_branch(target),
  };

  if !is_kind_match {
    return;
  }

  log!(INFO, "[+] {target}");

  match is_async {
    true => run_async(commands),
    false => run_sync(commands),
  }

  log!(INFO, "[-] {target}");
}

fn main() {
  const START: &str = "EC_";
  const ASYNC: &str = "ASYNC_";
  const SPACE: &str = "_";
  const DELIM: &str = ",";

  for (key, value) in vars() {
    let Some(key) = key.strip_prefix(START) else {
      continue;
    };

    let is_async = key.starts_with(ASYNC);
    let key = key.strip_prefix(ASYNC).unwrap_or(key);

    let mut parts = key.split(SPACE);
    let (kind, target) = (parts.next(), parts.next());

    let (Some(kind), Some(target)) = (kind.and_then(kind_from_str), target) else {
      log!(ERROR, "unrecognised format: {key}");
      continue;
    };

    let commands = value.split(DELIM).collect();
    start(kind, target, commands, is_async);
  }
}
