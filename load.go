// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-marshal/marshal authors

package marshal

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
)

// ErrShort is reported when the input ends in the middle of a value.
var ErrShort = errors.New("marshal: unexpected end of input")

// Load decodes a Ruby Marshal (version 4.8) byte stream into a Value. It returns
// an error for a truncated stream, an unsupported version, or an unknown type
// tag.
func Load(b []byte) (v Value, err error) {
	// Internal decode errors are signalled by panicking a marshalError; recover
	// converts it to a return value. Any other panic (a genuine bug) fails the
	// type assertion and propagates, so it is never silently swallowed.
	defer func() {
		if r := recover(); r != nil {
			v, err = nil, r.(marshalError)
		}
	}()
	d := &decoder{b: b}
	if len(b) < 2 {
		return nil, ErrShort
	}
	if b[0] != majorVersion || b[1] != minorVersion {
		return nil, fmt.Errorf("marshal: unsupported version %d.%d", b[0], b[1])
	}
	d.pos = 2
	return d.readValue(), nil
}

// marshalError wraps the error a decode panic carries. It unwraps to ErrShort
// for truncated input so callers can match it with errors.Is.
type marshalError struct{ err error }

func (m marshalError) Error() string { return m.err.Error() }
func (m marshalError) Unwrap() error { return m.err }

// fail panics with a formatted decode error.
func fail(format string, a ...any) { panic(marshalError{fmt.Errorf(format, a...)}) }

type decoder struct {
	b    []byte
	pos  int
	syms []string // symbol table, in definition order
	objs []Value  // object-link table, in definition order
}

// byteAt reads one byte, mapping past-the-end reads to ErrShort.
func (d *decoder) next() byte {
	if d.pos >= len(d.b) {
		panic(marshalError{ErrShort})
	}
	c := d.b[d.pos]
	d.pos++
	return c
}

// take reads n bytes.
func (d *decoder) take(n int) []byte {
	if n < 0 || d.pos+n > len(d.b) {
		panic(marshalError{ErrShort})
	}
	s := d.b[d.pos : d.pos+n]
	d.pos += n
	return s
}

// reserve appends a placeholder object slot (so cycles can refer to a container
// before its contents are read) and returns its index.
func (d *decoder) reserve() int {
	d.objs = append(d.objs, nil)
	return len(d.objs) - 1
}

func (d *decoder) readValue() Value {
	switch t := d.next(); t {
	case '0':
		return Nil{}
	case 'T':
		return Bool(true)
	case 'F':
		return Bool(false)
	case 'i':
		return Int{big.NewInt(int64(d.readLong()))}
	case 'l':
		return d.readBignum()
	case 'f':
		return d.readFloat()
	case ':':
		name := string(d.readRawString())
		d.syms = append(d.syms, name)
		return Symbol(name)
	case ';':
		idx := d.readLong()
		if idx < 0 || idx >= len(d.syms) {
			fail("marshal: symbol link out of range")
		}
		return Symbol(d.syms[idx])
	case '"':
		return d.readString(ASCII8BIT, "")
	case 'I':
		return d.readIVar()
	case '[':
		return d.readArray()
	case '{':
		return d.readHash(false)
	case '}':
		return d.readHash(true)
	case '@':
		idx := d.readLong()
		if idx < 0 || idx >= len(d.objs) || d.objs[idx] == nil {
			fail("marshal: object link out of range")
		}
		return d.objs[idx]
	default:
		panic(marshalError{fmt.Errorf("marshal: unsupported type tag %#x", t)})
	}
}

func (d *decoder) readBignum() Value {
	id := d.reserve()
	sign := d.next()
	n := d.readLong() * 2 // bytes = shorts * 2
	le := d.take(n)
	be := make([]byte, n)
	for i := 0; i < n; i++ {
		be[n-1-i] = le[i]
	}
	z := new(big.Int).SetBytes(be)
	if sign == '-' {
		z.Neg(z)
	}
	v := Int{z}
	d.objs[id] = v
	return v
}

func (d *decoder) readFloat() Value {
	id := d.reserve()
	v := Float(parseRubyFloat(string(d.readRawString())))
	d.objs[id] = v
	return v
}

func (d *decoder) readString(enc Encoding, name string) *Str {
	id := d.reserve()
	raw := d.readRawString()
	s := &Str{Bytes: append([]byte(nil), raw...), Enc: enc, Name: name}
	d.objs[id] = s
	return s
}

// readIVar handles 'I' (an object carrying instance variables). For the subset
// here it always wraps a String; the only ivars are the encoding markers
// (E => true/false, or encoding => <name>).
func (d *decoder) readIVar() Value {
	if d.next() != '"' {
		fail("marshal: only String instance-var objects are supported")
	}
	id := d.reserve()
	raw := d.readRawString()
	s := &Str{Bytes: append([]byte(nil), raw...), Enc: UTF8}
	d.objs[id] = s

	for n := d.readLong(); n > 0; n-- {
		key, ok := d.readValue().(Symbol)
		if !ok {
			fail("marshal: instance-variable name is not a Symbol")
		}
		val := d.readValue()
		switch key {
		case "E":
			if val == Bool(true) {
				s.Enc = UTF8
			} else {
				s.Enc = USASCII
			}
		case "encoding":
			ev, ok := val.(*Str)
			if !ok {
				fail("marshal: encoding value is not a String")
			}
			s.Enc = Named
			s.Name = string(ev.Bytes)
		}
	}
	return s
}

func (d *decoder) readArray() Value {
	id := d.reserve()
	a := &Array{}
	d.objs[id] = a
	for n := d.readLong(); n > 0; n-- {
		a.Elems = append(a.Elems, d.readValue())
	}
	return a
}

func (d *decoder) readHash(withDefault bool) Value {
	id := d.reserve()
	h := &Hash{}
	d.objs[id] = h
	for n := d.readLong(); n > 0; n-- {
		k := d.readValue()
		v := d.readValue()
		h.Keys = append(h.Keys, k)
		h.Vals = append(h.Vals, v)
	}
	if withDefault {
		h.Default = d.readValue()
	}
	return h
}

// readRawString reads a length-prefixed byte string.
func (d *decoder) readRawString() []byte {
	return d.take(d.readLong())
}

// readLong decodes Ruby's packed "long" (marshal's r_long).
func (d *decoder) readLong() int {
	c := int(int8(d.next()))
	switch {
	case c == 0:
		return 0
	case c > 0:
		if c >= 5 {
			return c - 5
		}
		n := 0
		for i := 0; i < c; i++ {
			n |= int(d.next()) << (8 * i)
		}
		return n
	default:
		if c <= -5 {
			return c + 5
		}
		cnt := -c
		n := -1
		for i := 0; i < cnt; i++ {
			n &^= 0xff << (8 * i)
			n |= int(d.next()) << (8 * i)
		}
		return n
	}
}

// parseRubyFloat parses a marshalled float string.
func parseRubyFloat(s string) float64 {
	switch s {
	case "inf":
		return math.Inf(1)
	case "-inf":
		return math.Inf(-1)
	case "nan":
		return math.NaN()
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
