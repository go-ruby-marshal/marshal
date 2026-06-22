// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-marshal/marshal authors

package marshal

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Marshal format version, emitted as the two-byte header 0x04 0x08.
const (
	majorVersion = 4
	minorVersion = 8
)

// Fixnum marshal range: integers in [fixnumMin, fixnumMax] use the compact 'i'
// form; everything outside uses the Bignum 'l' form (this is MRI's boundary,
// not the platform word size).
const (
	fixnumMin = -(1 << 30)
	fixnumMax = (1 << 30) - 1
)

// Dump returns the Ruby Marshal (version 4.8) encoding of v.
func Dump(v Value) []byte {
	e := &encoder{syms: map[string]int{}, objs: map[Value]int{}}
	e.buf = append(e.buf, majorVersion, minorVersion)
	e.writeValue(v)
	return e.buf
}

type encoder struct {
	buf    []byte
	syms   map[string]int // symbol name -> symbol-table index
	objs   map[Value]int  // composite-pointer identity -> object-link index
	nextID int            // next object-link index to assign
}

// newID consumes and returns the next object-link index.
func (e *encoder) newID() int { id := e.nextID; e.nextID++; return id }

// link records p under a fresh object id and returns false; or, if p was seen
// before, emits the '@' link to its id and returns true.
func (e *encoder) link(p Value) bool {
	if id, ok := e.objs[p]; ok {
		e.buf = append(e.buf, '@')
		e.writeLong(id)
		return true
	}
	e.objs[p] = e.newID()
	return false
}

func (e *encoder) writeValue(v Value) {
	switch x := v.(type) {
	case Nil:
		e.buf = append(e.buf, '0')
	case Bool:
		if x {
			e.buf = append(e.buf, 'T')
		} else {
			e.buf = append(e.buf, 'F')
		}
	case Symbol:
		e.writeSymbol(string(x))
	case Int:
		e.writeInt(x.I)
	case Float:
		e.newID()
		e.writeFloat(float64(x))
	case *Str:
		if !e.link(x) {
			e.writeStr(x)
		}
	case *Array:
		if !e.link(x) {
			e.buf = append(e.buf, '[')
			e.writeLong(len(x.Elems))
			for _, el := range x.Elems {
				e.writeValue(el)
			}
		}
	case *Hash:
		if !e.link(x) {
			e.writeHash(x)
		}
	default:
		panic(fmt.Sprintf("marshal: cannot dump %T", v))
	}
}

func (e *encoder) writeInt(b *big.Int) {
	if b.IsInt64() {
		if n := b.Int64(); n >= fixnumMin && n <= fixnumMax {
			e.buf = append(e.buf, 'i')
			e.writeLong(int(n))
			return
		}
	}
	e.newID() // a Bignum is a linkable object
	e.buf = append(e.buf, 'l')
	if b.Sign() < 0 {
		e.buf = append(e.buf, '-')
	} else {
		e.buf = append(e.buf, '+')
	}
	le := littleEndianAbs(b)
	e.writeLong(len(le) / 2) // count in 16-bit shorts
	e.buf = append(e.buf, le...)
}

// littleEndianAbs returns |b| as little-endian bytes, padded to an even length.
func littleEndianAbs(b *big.Int) []byte {
	be := new(big.Int).Abs(b).Bytes() // big-endian, no leading zeros
	le := make([]byte, len(be))
	for i, by := range be {
		le[len(be)-1-i] = by
	}
	if len(le)%2 == 1 {
		le = append(le, 0)
	}
	return le
}

func (e *encoder) writeFloat(f float64) {
	e.buf = append(e.buf, 'f')
	e.writeRawString(floatToRubyString(f))
}

func (e *encoder) writeStr(s *Str) {
	if s.Enc == ASCII8BIT {
		e.buf = append(e.buf, '"')
		e.writeRawString(string(s.Bytes))
		return
	}
	e.buf = append(e.buf, 'I', '"')
	e.writeRawString(string(s.Bytes))
	switch s.Enc {
	case UTF8:
		e.writeLong(1)
		e.writeSymbol("E")
		e.buf = append(e.buf, 'T')
	case USASCII:
		e.writeLong(1)
		e.writeSymbol("E")
		e.buf = append(e.buf, 'F')
	default: // Named
		e.writeLong(1)
		e.writeSymbol("encoding")
		e.writeValue(&Str{Bytes: []byte(s.Name), Enc: ASCII8BIT})
	}
}

func (e *encoder) writeHash(h *Hash) {
	if h.Default != nil {
		e.buf = append(e.buf, '}')
	} else {
		e.buf = append(e.buf, '{')
	}
	e.writeLong(len(h.Keys))
	for i := range h.Keys {
		e.writeValue(h.Keys[i])
		e.writeValue(h.Vals[i])
	}
	if h.Default != nil {
		e.writeValue(h.Default)
	}
}

func (e *encoder) writeSymbol(name string) {
	if id, ok := e.syms[name]; ok {
		e.buf = append(e.buf, ';')
		e.writeLong(id)
		return
	}
	e.syms[name] = len(e.syms)
	e.buf = append(e.buf, ':')
	e.writeRawString(name)
}

// writeRawString emits a length-prefixed byte string (no type tag).
func (e *encoder) writeRawString(s string) {
	e.writeLong(len(s))
	e.buf = append(e.buf, s...)
}

// writeLong emits n in Ruby's packed "long" encoding (marshal's w_long), used
// for Fixnum values and for every length/index/count in the format.
func (e *encoder) writeLong(n int) {
	if n == 0 {
		e.buf = append(e.buf, 0)
		return
	}
	if n > 0 && n < 123 {
		e.buf = append(e.buf, byte(n+5))
		return
	}
	if n < 0 && n > -124 {
		e.buf = append(e.buf, byte(n-5))
		return
	}
	// Emit the value bytes little-endian, prefixed by the signed byte count. The
	// loop always terminates: under arithmetic right shift every positive value
	// reaches 0 and every negative value reaches -1. Callers stay within the
	// Fixnum range (±2**30), so this never exceeds four value bytes.
	var tmp []byte
	for i := 1; ; i++ {
		tmp = append(tmp, byte(n&0xff))
		n >>= 8
		if n == 0 {
			e.buf = append(e.buf, byte(i))
			e.buf = append(e.buf, tmp...)
			return
		}
		if n == -1 {
			e.buf = append(e.buf, byte(256-i))
			e.buf = append(e.buf, tmp...)
			return
		}
	}
}

// floatToRubyString formats f the way MRI's marshal does: the shortest decimal
// that round-trips, using exponent notation only when the decimal point falls
// before the fourth place to the left or past the last significant digit.
func floatToRubyString(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}

	// strconv 'e' with -1 precision yields the shortest round-tripping mantissa,
	// one digit before the point: "d[.ddd]e±XX".
	s := strconv.FormatFloat(f, 'e', -1, 64)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	ei := strings.IndexByte(s, 'e')
	mant := s[:ei]
	exp, _ := strconv.Atoi(s[ei+1:])
	var digits string
	if dot := strings.IndexByte(mant, '.'); dot >= 0 {
		digits = mant[:dot] + mant[dot+1:]
	} else {
		digits = mant
	}
	decpt := exp + 1
	digs := len(digits)

	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	switch {
	case decpt < -3 || decpt > digs:
		b.WriteByte(digits[0])
		if digs > 1 {
			b.WriteByte('.')
			b.WriteString(digits[1:])
		}
		b.WriteByte('e')
		b.WriteString(strconv.Itoa(decpt - 1))
	case decpt > 0:
		if decpt >= digs {
			b.WriteString(digits)
			b.WriteString(strings.Repeat("0", decpt-digs))
		} else {
			b.WriteString(digits[:decpt])
			b.WriteByte('.')
			b.WriteString(digits[decpt:])
		}
	default:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -decpt))
		b.WriteString(digits)
	}
	return b.String()
}
