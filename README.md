Environment-specific and configurable commands.

## Description

Basic utility for aliasing commands across environments (directories and
branches). Used at work to avoid rewriting the same setup and teardown commands
across multiple services.

## Installation

```sh
brew install nullbrna/tap/envcmd
```

## Usage

1. Set environment variables in the following format:

```sh
EVC_DIR_ENVCMD="echo 'foo' ||| echo 'bar'"
EVC_BRA_MAIN="echo 'bar' ||| echo 'foo'"
EVC_ALL_ENVCMD__MAIN="echo 'foobar' ||| echo 'barfoo'"
```

| Part                | Description                                          |
| ------------------- | ---------------------------------------------------- |
| **DIR / BRA / ALL** | Environment type to match against.                   |
| **TARGET**          | Directory or branch name to run the commands within. |

> NOTE: `TARGET` will have some characters (i.e. `-`, `.`, `/` characters)
> replaced to find a matching environment variable.

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
git tag vX.X.X
git push origin vX.X.X
```

2. A GitHub Action will build the archive and generate its SHA-256 checksum.
   Copy the checksum, along with the new version number and archive name, to
   update the [corresponding
   tap](https://github.com/nullbrna/homebrew-tap/blob/main/Formula/envcmd.rb)
   metadata.
