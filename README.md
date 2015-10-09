# proj2aci

`proj2aci` is a command-line tool to build various projects into ACIs which conform to the [app container specification][appc-spec].

Currently `proj2aci` supports rather simple cases:

- Go projects buildable with `go get`
- Rather simpler CMake projects

`proj2aci` project is probably going to be superseded by [acbuild]
project. Also, the UI of the `proj2aci` application and the API of the
`proj2aci` library are not stable.

[appc-spec]: https://github.com/appc/spec
[acbuild]: https://github.com/appc/acbuild

## Usage

### `proj2aci go`

The simplest invocation of `proj2aci go` would be: `proj2aci go github.com/coreos/etcd`

For additional parameters, please call `proj2aci go --help`.

### `proj2aci cmake`

The simplest invocation of `proj2aci go` would be: `proj2aci cmake github.com/cmake-stuff/project`

For additional parameters, please call `proj2aci cmake --help`.

### a library

`github.com/appc/proj2aci/proj2aci` provides a `Builder` type which
take `BuilderCustomizations` interface implementation. The `Builder`
is doing all the heavy lifting, while `BuilderCustomizations` provide
some specific bits for builder. The library provides
`BuilderCustomizations` for `go` and `cmake` projects. A developer can
provide another implementation of `BuilderCustomizations` for building
a different kind of a project.

Besides the above, the library provides also some other useful
functions and types for assets preparing or getting vcs info.

## TODO

Lots, check out https://github.com/appc/proj2aci/issues
