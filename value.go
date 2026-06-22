// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-marshal/marshal authors

package marshal

import "math/big"

// Value is a Ruby value in the subset this package serializes. The concrete
// types are Nil, Bool, Int, Float, Symbol, *Str, *Array, and *Hash; RubyClass
// reports the name of the corresponding Ruby class.
//
// The composite, mutable types (*Str, *Array, *Hash) are pointers so that
// identity is observable: the same pointer appearing more than once in a
// structure is encoded once and thereafter as an object link, exactly as MRI
// does, which is also what makes cyclic structures representable.
type Value interface{ RubyClass() string }

// Nil is Ruby nil.
type Nil struct{}

// Bool is Ruby true / false.
type Bool bool

// Int is a Ruby Integer of any magnitude. Dump emits the compact Fixnum form
// for values in [-2**30, 2**30-1] (matching MRI's marshal Fixnum range) and the
// Bignum form otherwise.
type Int struct{ I *big.Int }

// Float is a Ruby Float.
type Float float64

// Symbol is a Ruby Symbol (its name, without the leading colon).
type Symbol string

// Encoding identifies a Ruby String's encoding for marshalling purposes.
type Encoding int

const (
	// UTF8 is the default; marshalled as the instance variable E => true.
	UTF8 Encoding = iota
	// USASCII is marshalled as E => false.
	USASCII
	// ASCII8BIT (BINARY) is marshalled as a bare String with no encoding ivar.
	ASCII8BIT
	// Named is any other encoding, marshalled as encoding => <Name>.
	Named
)

// Str is a Ruby String: raw bytes plus an encoding. The zero value is an empty
// UTF-8 string. For a Named encoding, Name holds the encoding's name.
type Str struct {
	Bytes []byte
	Enc   Encoding
	Name  string // only used when Enc == Named
}

// Array is a Ruby Array.
type Array struct{ Elems []Value }

// Hash is a Ruby Hash. Keys and Vals are parallel slices preserving insertion
// order (as Ruby hashes do). Default, when non-nil, is the hash's default value
// (Hash.new(default)); it is marshalled with the TYPE_HASH_DEF tag.
type Hash struct {
	Keys    []Value
	Vals    []Value
	Default Value
}

// RubyClass reports the Ruby class name of the value.
func (Nil) RubyClass() string { return "NilClass" }

// RubyClass reports the Ruby class name of the value.
func (b Bool) RubyClass() string {
	if b {
		return "TrueClass"
	}
	return "FalseClass"
}

// RubyClass reports the Ruby class name of the value.
func (Int) RubyClass() string { return "Integer" }

// RubyClass reports the Ruby class name of the value.
func (Float) RubyClass() string { return "Float" }

// RubyClass reports the Ruby class name of the value.
func (Symbol) RubyClass() string { return "Symbol" }

// RubyClass reports the Ruby class name of the value.
func (*Str) RubyClass() string { return "String" }

// RubyClass reports the Ruby class name of the value.
func (*Array) RubyClass() string { return "Array" }

// RubyClass reports the Ruby class name of the value.
func (*Hash) RubyClass() string { return "Hash" }

// NewInt returns an Int holding n.
func NewInt(n int64) Int { return Int{big.NewInt(n)} }

// NewString returns a UTF-8 *Str holding s.
func NewString(s string) *Str { return &Str{Bytes: []byte(s), Enc: UTF8} }
