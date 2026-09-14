Environment-specific and configurable commands.

## Description

Basic program to alias common commands between environments (i.e. directories
and branches). Mainly used at work to avoid writing the same setup/teardown
commands across services.

## Installation

```sh
brew install nullbrna/tap/envcmd
```

## Usage

1. Set environment variables like the following:

```sh
# Only runs in the directory "envcmd" (case-insensitive).
EVC_DIR_ENVCMD="echo 'foo' ||| echo 'bar'",

# Only runs on the Git branch "main" (case-insensitive).
EVC_BRA_MAIN="echo 'bar' ||| echo 'foo'",

# Only runs when BOTH the above conditions are met.
EVC_ALL_ENVCMD__MAIN="echo 'foobar' ||| echo 'barfoo'",
```

| Key                 | Description                                            |
| ------------------- | ------------------------------------------------------ |
| **DIR / BRA / ALL** | Directory, branch or both to run commands when matched |
| **TARGET**          | The "matcher" to compare the environment name against  |

> NOTE: Characters like "-", ".", "/" will be replaced with underscores to match
> the `TARGET` value set in your environment.

2. No subcommands available. Just run directly:

```sh
envcmd
```

## Release

1. Push the latest changes, then push up a new tag:

```sh
git tag vX.X.X
git push origin vX.X.X
```

2. An action will run to build and create a hash. Copy said hash along with the
   new version number to update the [corresponding
   tap](https://github.com/nullbrna/homebrew-tap/blob/main/Formula/envcmd.rb)
   metadata.
