# goaci

`goaci` is a simple command-line tool to build go projects into ACIs which confirm to the [app container specification][appc-spec].

[appc-spec]: https://github.com/appc/spec

## Usage

Use goaci as you would `go get`:

	$ goaci github.com/coreos/etcd
	Wrote etcd.aci
	$ actool -debug validate etcd.aci
	etcd.aci: valid app container image

## How it works

`goaci` creates a temporary directory and uses it as a `GOPATH`; it then `go get`s the specified package and compiles it statically.
Then it generates a very basic image manifest (using mostly default values, configurables coming soon) and leverages the [appc/spec](https://github.com/appc/spec) libraries to construct an ACI.

## TODO

Lots, check out https://github.com/appc/goaci/issues
