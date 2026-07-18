Environment-specific and configurable commands.

## Description

Basic utility for aliasing commands across environments (directories and
branches), with optional asynchronous execution. Used at work to avoid rewriting
the same setup and teardown commands across multiple services.

## Installation

```sh
brew install nullbrna/tap/envcmd
```

## Usage

1. Set environment variables in the following format:

```
EVC_[ASYNC_]<DIR|BRA>_<TARGET>="echo 'foo' ||| echo 'bar'"
```

| Part                   | Description                                          |
|------------------------|------------------------------------------------------|
| **ASYNC** (optional)   | Runs all the commands concurrently.                  |
| **DIR / BRA**          | Environment type to match against.                   |
| **TARGET**             | Directory or branch name to run the commands within. |

2. Run the command:

```sh
envcmd
```

## Release

1. Push the latest changes, then create and push a new tag:

```sh
# View the existing tags.
git tag

# Create and push to remote.
git tag v0.0.0
git push origin v0.0.0
```

2. A GitHub Action will build the archive and generate its SHA-256 checksum.
   Copy the checksum, along with the new version number and archive name, to
   update the [corresponding
   tap](https://github.com/nullbrna/homebrew-tap/blob/main/Formula/envcmd.rb)
   metadata.
