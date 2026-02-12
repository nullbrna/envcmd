use std::io::BufRead;
use std::io::BufReader;
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
    println!("{}{}.\x1b[0m {}", COLOUR[$idx % COLOUR.len()], $idx, format_args!($($arg)*));
  }};
  (ABORT, $($arg:tt)*) => {{
    eprintln!("\x1b[31mE.\x1b[0m {}", format_args!($($arg)*));
    std::process::exit(1);
  }};
}

fn run_command(idx: usize, cmd: &str) {
  let merged_cmd = format!("{cmd} 2>&1");
  log!(COLOUR, idx, "[+] {cmd}");

  let Ok(mut child) = Command::new("sh")
    .arg("-c")
    .arg(&merged_cmd)
    .stdout(Stdio::piped())
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

  log!(COLOUR, idx, "[-] {cmd}");
}

fn main() {
  run_command(0, "echo 'hello, world!'");
  run_command(1, "echo 'goodbye, world!'");
}
