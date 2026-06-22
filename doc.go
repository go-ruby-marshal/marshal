// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-marshal/marshal authors

// Package marshal is a pure-Go (CGO=0) implementation of Ruby's Marshal binary
// serialization format — the wire format produced by Ruby's Marshal.dump and
// consumed by Marshal.load, version 4.8.
//
// It operates on its own small, typed value model ([Value]) rather than on any
// particular interpreter's objects, so it is reusable on its own and can be
// bridged into an embedded Ruby (e.g. go-embedded-ruby) by converting that
// interpreter's values to and from this package's.
//
// Supported types: nil, true, false, Integer (Fixnum and arbitrary-precision
// Bignum), Float, Symbol, String (UTF-8 / US-ASCII / ASCII-8BIT encodings),
// Array, and Hash (including a default value). The format's symbol table and
// object-link table are implemented, so repeated symbols, shared mutable
// objects, and cyclic structures round-trip exactly as in MRI.
//
// The byte output is verified equal to MRI Ruby's Marshal.dump (differential
// tests against the reference interpreter).
package marshal
