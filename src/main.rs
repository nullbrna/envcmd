use std::ffi::OsStr;
use std::io::BufRead;
use std::io::BufReader;
use std::process::Command;
use std::process::ExitStatus;
use std::process::Stdio;

const VERSION: &str = env!("CARGO_PKG_VERSION");

struct Context {
    directory: Option<String>,
    branch: Option<String>,
}

impl Context {
    fn new() -> Self {
        let directory = std::env::current_dir()
            .unwrap_or_default()
            .file_name()
            .and_then(OsStr::to_str)
            .filter(|name| name.len() > 0)
            .map(Self::normalise);

        let output = Command::new("git")
            .args(["branch", "--show-current"])
            .output()
            .map(|output| output.stdout)
            .unwrap_or_default();

        let branch = String::from_utf8(output)
            .ok()
            .as_deref()
            .map(Self::normalise)
            .filter(|output| output.len() > 0);

        Self { directory, branch }
    }

    fn values(&self) -> [Option<String>; 3] {
        let mut values = [None, None, None];

        if let Some(directory) = &self.directory {
            let key = format!("EVC_DIR_{directory}");
            values[0] = std::env::var(key).ok();
        }

        if let Some(branch) = &self.branch {
            let key = format!("EVC_BRA_{branch}");
            values[1] = std::env::var(key).ok();
        }

        if let Some(directory) = &self.directory
            && let Some(branch) = &self.branch
        {
            let key = format!("EVC_ALL_{directory}__{branch}");
            values[2] = std::env::var(key).ok();
        }

        values
    }

    fn normalise(value: &str) -> String {
        let cap = value.len();
        let mut builder = String::with_capacity(cap);

        value.chars().for_each(|ch| {
            if ch.is_ascii_alphanumeric() {
                let upper = ch.to_ascii_uppercase();
                builder.push(upper);
            } else if !builder.ends_with('_') {
                builder.push('_');
            }
        });

        builder.trim_end_matches('_').to_owned()
    }
}

fn run_command(index: usize, command: &str) {
    println!("\x1b[90m→\x1b[22m {command}\x1b[0m");

    let abort = |msg: &str, command: &str| -> ! {
        eprint!("[\x1b[31mERR\x1b[0m] {} ", msg);
        eprintln!("\x1b[90mcommand=\"{}\"\x1b[0m", command);
        std::process::exit(1);
    };

    let runner = format!("{command} 2>&1");
    let pipe = Stdio::piped();

    let Ok(mut child) = Command::new("sh")
        .arg("-c")
        .arg(runner)
        .stdout(pipe)
        .spawn()
    else {
        abort("Starting the process", command);
    };

    let Some(stdout) = child.stdout.take() else {
        abort("Reading stream output", command);
    };

    let mut reader = BufReader::new(stdout);
    let mut buffer = String::with_capacity(256);

    static COLOURS: [u8; 5] = [32, 33, 34, 35, 36];
    let colour = COLOURS[index % COLOURS.len()];

    while reader.read_line(&mut buffer).is_ok_and(|count| count > 0) {
        print!("[\x1b[{colour}m{index}\x1b[0m] {buffer}");
        buffer.clear();
    }

    if !child.wait().as_ref().is_ok_and(ExitStatus::success) {
        abort("Non-zero exit code", command);
    }

    println!("\x1b[90m←\x1b[22m {command}\x1b[0m");
}

fn main() {
    let context = Context::new();

    context.values().into_iter().flatten().for_each(|value| {
        let mut index = 1;
        let values = value
            .split("|||")
            .map(str::trim)
            .filter(|command| command.len() > 0);

        values.for_each(|command| {
            run_command(index, command);
            index += 1;
        });
    });

    println!("\nenvcmd@v{VERSION}");
}
