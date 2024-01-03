# vgopath

`vgopath` is a tool for module-enabled projects to set up a 'virtual' GOPATH for
legacy tools to run with (`kubernetes/code-generator` I'm looking at you...).

## Installation

The simplest way to install `vgopath` is by running

```shell
go install github.com/ironcore-dev/vgopath@latest
```

## Usage

`vgopath` has to be run from the module-enabled project root. It requires a
target directory to construct the virtual GOPATH.

Example usage could look like this:

```shell
# Create the target directory
mkdir -p my-vgopath

# Do the linking in my-vgopath
vgopath my-vgopath
```

Once done, the structure will look something like

```
my-vgopath
├── bin -> <GOPATH>/bin
├── pkg -> <GOPATH>/pkg
└── src -> various subdirectories
```
