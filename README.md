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

## Release

1. Push the latest changes. Then push up a new tag:

```sh
# View the existing tags.
git tag

git tag v0.0.0
git push origin v0.0.0
```

2. An action will run to build and create a hash. Copy said hash along with the
   new version number & file name to update the [corresponding
   tap](https://github.com/nullbrna/homebrew-tap/blob/main/Formula/envcmd.rb)
   metadata.
