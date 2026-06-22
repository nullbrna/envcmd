Environment-specific and configurable commands.

## Description

Basic program to alias common commands between environments (directories and
branches) with asynchronous capabilities. Mainly used at work to avoid writing
the same setup/teardown commands across services over-and-over.

## Installation

```sh
brew install nullbrna/tap/envcmd
```

## Usage

1. Set environment variables prefixed with the following format:

```
EVC_[ASYNC_]<DIR|BRA>_<TARGET>
```

- **ASYNC:** Runs all the commands concurrently (optional)
- **DIR / BRA:** Kind of environment to check for a match
- **TARGET:** The directory or branch name to, if matching, run the commands within

2. No subcommands available. Just run directly:

```sh
envcmd
```

## Release

1. Push the latest changes, then push up a new tag:

```sh
# View the existing tags.
git tag

git tag v0.0.0
git push origin v0.0.0
```

2. An action will run to build and create a hash. Copy said hash along with the
   new version number and file name to update the [corresponding
   tap](https://github.com/nullbrna/homebrew-tap/blob/main/Formula/envcmd.rb)
   metadata.
