# RDI Live
## Description
RDI Live is a project for live aggregation and display of streaming data.  It
was built as an internal development tool and uses
[proio](https://github.com/proio-org) streams gathered continuously from
radiation detectors.

This material is based upon work supported by the U.S. Department of Energy,
Office of Science, Nuclear Physics program office under Award Number
DE-SC0015136.
## Screenshots
![RDI logo beam scan](images/screenshot1.png)
## Tools installation
The tools include the main `rdi-live` binary.
1. Install the Go compiler toolchain (version 1.11 or newer) from
   [https://golang.org/dl/](https://golang.org/dl).
2. `go get` the tools directory recursively
```shell
go get github.com/rditech/rdi-live/tools/...
```
