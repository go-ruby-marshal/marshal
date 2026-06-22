# go-ruby-marshal/marshal

[![Go Reference](https://pkg.go.dev/badge/github.com/go-ruby-marshal/marshal.svg)](https://pkg.go.dev/github.com/go-ruby-marshal/marshal)
[![License: BSD-3-Clause](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![CI](https://github.com/go-ruby-marshal/marshal/actions/workflows/ci.yml/badge.svg)](https://github.com/go-ruby-marshal/marshal/actions/workflows/ci.yml)

A pure-Go (**CGO=0**) implementation of Ruby's **Marshal** binary serialization
format — the wire format produced by `Marshal.dump` and consumed by
`Marshal.load`, version 4.8. Its byte output is verified **equal to MRI Ruby's**
(differential-tested against the reference interpreter, Ruby 4.0.5).

It is part of the `go-ruby-*` family of standalone front-end/runtime components
(alongside [go-ruby-parser](https://github.com/go-ruby-parser/parser) and
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp)) that
[go-embedded-ruby](https://github.com/go-embedded-ruby) builds on, but it has no
dependency on any interpreter: it works against its own small, typed value model
and can be bridged into one by converting values at the boundary.

## Supported

- **nil**, **true**, **false**
- **Integer** — both the compact Fixnum form (`[-2³⁰, 2³⁰-1]`, matching MRI's
  marshal range) and arbitrary-precision **Bignum**
- **Float** — the shortest round-tripping decimal, formatted exactly as MRI does
  (`1.0` → `1`, `100.0` → `1e2`, `±inf`, `nan`, `-0`)
- **Symbol** — with the format's symbol table (repeated symbols become links)
- **String** — UTF-8, US-ASCII, and ASCII-8BIT (BINARY), plus other named
  encodings
- **Array**, **Hash** (including a default value, `Hash.new(d)`)
- **Object links** — shared mutable objects are encoded once and thereafter as
  links, so cyclic structures (`a = []; a << a`) round-trip exactly as in MRI

## Usage

```go
import "github.com/go-ruby-marshal/marshal"

// Encode  →  Ruby Marshal bytes (Marshal.dump compatible)
b := marshal.Dump(&marshal.Array{Elems: []marshal.Value{
    marshal.NewInt(1), marshal.NewString("hi"), marshal.Bool(true),
}})

// Decode  ←  bytes produced by Ruby's Marshal.dump
v, err := marshal.Load(b)
```

## License

BSD-3-Clause.
