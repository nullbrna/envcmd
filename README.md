Environment-specific and configurable commands.

## Description

Basic program to alias common commands between environments (directories &
branches) with asynchronous capabilities. Mainly used at work to avoid writing
the same setup/teardown commands across services over-and-over.

## Usage

No subcommands available, just run directly:

```sh
envcmd
```

Set environment variables prefixed with the following format:

```
EVC_[ASYNC_]<DIR|BRA>_<TARGET>
```

- **ASYNC:** Optional, runs all the commands concurrently
- **DIR / BRA:** Directory or branch name (respectively) to run commands within
- **TARGET:** The "matcher" to compare the environment name against
