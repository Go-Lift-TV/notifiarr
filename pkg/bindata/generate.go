// Package bindata provides the go:generate command to create new base64 binary
// data, as well as the binary data itself, in both formats. That means, you will
// find the _things_ we compress into base64 inside the files/ directory, and you
// will find the base64 files in the bindata.go file. The generate.go file contains
// the command that creates the binary data.
// See: https://github.com/kevinburke/go-bindata or the README.md file.
package bindata

//go:generate go run github.com/kevinburke/go-bindata/v4/go-bindata@latest -pkg bindata -modtime 1587356420 -o bindata.go files/... templates/... other/...
