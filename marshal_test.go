// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-marshal/marshal authors

package marshal

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"
)

// hx decodes a space-separated hex string into bytes.
func hx(s string) []byte {
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		panic(err)
	}
	return b
}

// golden pairs a Value with MRI's exact Marshal.dump bytes (captured from Ruby
// 4.0.5). These run without the interpreter so coverage is reproducible on every
// CI architecture.
var golden = []struct {
	name  string
	val   Value
	bytes string
}{
	{"nil", Nil{}, "04 08 30"},
	{"true", Bool(true), "04 08 54"},
	{"false", Bool(false), "04 08 46"},
	{"zero", NewInt(0), "04 08 69 00"},
	{"122", NewInt(122), "04 08 69 7f"},
	{"123", NewInt(123), "04 08 69 01 7b"},
	{"300", NewInt(300), "04 08 69 02 2c 01"},
	{"70000", NewInt(70000), "04 08 69 03 70 11 01"},
	{"neg1", NewInt(-1), "04 08 69 fa"},
	{"neg123", NewInt(-123), "04 08 69 80"},
	{"neg300", NewInt(-300), "04 08 69 fe d4 fe"},
	{"maxfix", NewInt(1073741823), "04 08 69 04 ff ff ff 3f"},
	{"minfix", NewInt(-1073741824), "04 08 69 fc 00 00 00 c0"},
	{"big2_30", NewInt(1073741824), "04 08 6c 2b 07 00 00 00 40"},
	{"float1.5", Float(1.5), "04 08 66 08 31 2e 35"},
	{"sym", Symbol("abc"), "04 08 3a 08 61 62 63"},
	{"symref", &Array{Elems: []Value{Symbol("abc"), Symbol("abc")}}, "04 08 5b 07 3a 08 61 62 63 3b 00"},
	{"strUTF8", NewString("hi"), "04 08 49 22 07 68 69 06 3a 06 45 54"},
	{"strEmpty", NewString(""), "04 08 49 22 00 06 3a 06 45 54"},
	{"strBinary", &Str{Bytes: []byte("hi"), Enc: ASCII8BIT}, "04 08 22 07 68 69"},
	{"strUSASCII", &Str{Bytes: []byte("hi"), Enc: USASCII}, "04 08 49 22 07 68 69 06 3a 06 45 46"},
	{"array", &Array{Elems: []Value{NewInt(1), NewInt(2), NewInt(3)}}, "04 08 5b 08 69 06 69 07 69 08"},
	{"arrayEmpty", &Array{}, "04 08 5b 00"},
	{"hash", &Hash{Keys: []Value{NewInt(1)}, Vals: []Value{NewInt(2)}}, "04 08 7b 06 69 06 69 07"},
	{"hashEmpty", &Hash{}, "04 08 7b 00"},
	{"hashDefault", &Hash{Default: NewInt(0)}, "04 08 7d 00 69 00"},
}

func TestGoldenDump(t *testing.T) {
	for _, g := range golden {
		if got, want := Dump(g.val), hx(g.bytes); !bytes.Equal(got, want) {
			t.Errorf("Dump(%s) = %x, want %x", g.name, got, want)
		}
	}
}

func TestGoldenLoadRoundTrip(t *testing.T) {
	for _, g := range golden {
		v, err := Load(hx(g.bytes))
		if err != nil {
			t.Errorf("Load(%s): %v", g.name, err)
			continue
		}
		if got := Dump(v); !bytes.Equal(got, hx(g.bytes)) {
			t.Errorf("round-trip %s: re-dump %x != %x", g.name, got, hx(g.bytes))
		}
	}
}

func TestBignumRoundTrip(t *testing.T) {
	for _, n := range []string{
		"1208925819614629174706176",   // 2**80
		"-1208925819614629174706176",  // -(2**80)
		"18446744073709551616",        // 2**64
		"123456789012345678901234567", // odd byte length (forces padding)
	} {
		z, _ := new(big.Int).SetString(n, 10)
		v := Int{z}
		out := Dump(v)
		back, err := Load(out)
		if err != nil {
			t.Fatalf("Load bignum %s: %v", n, err)
		}
		if back.(Int).I.Cmp(z) != 0 {
			t.Errorf("bignum %s round-trip = %s", n, back.(Int).I)
		}
	}
}

func TestFloatFormat(t *testing.T) {
	for _, c := range []struct {
		f    float64
		want string
	}{
		{1.0, "1"}, {2.0, "2"}, {1.5, "1.5"}, {10.0, "1e1"}, {12.0, "12"},
		{100.0, "1e2"}, {123456789.0, "123456789"}, {1234.5, "1234.5"},
		{1e16, "1e16"}, {1e20, "1e20"}, {9.999e22, "9.999e22"},
		{0.1, "0.1"}, {0.01, "0.01"}, {0.001, "0.001"}, {0.0001, "0.0001"},
		{1e-5, "1e-5"}, {1e-10, "1e-10"},
		{-2.5, "-2.5"}, {math.Copysign(0, -1), "-0"},
		{math.Inf(1), "inf"}, {math.Inf(-1), "-inf"}, {math.NaN(), "nan"},
	} {
		if got := floatToRubyString(c.f); got != c.want {
			t.Errorf("floatToRubyString(%v) = %q, want %q", c.f, got, c.want)
		}
	}
	// +0.0 (not negative zero)
	if got := floatToRubyString(0.0); got != "0" {
		t.Errorf("floatToRubyString(0) = %q", got)
	}
}

func TestFloatLoad(t *testing.T) {
	for _, c := range []struct {
		bytes string
		want  float64
	}{
		{"04 08 66 08 31 2e 35", 1.5},
		{"04 08 66 08 69 6e 66", math.Inf(1)},
		{"04 08 66 09 2d 69 6e 66", math.Inf(-1)},
	} {
		v, err := Load(hx(c.bytes))
		if err != nil {
			t.Fatalf("Load float: %v", err)
		}
		if f := float64(v.(Float)); f != c.want {
			t.Errorf("Load float = %v, want %v", f, c.want)
		}
	}
	// NaN loads to NaN.
	v, _ := Load(hx("04 08 66 08 6e 61 6e"))
	if !math.IsNaN(float64(v.(Float))) {
		t.Error("nan did not load as NaN")
	}
	if !math.IsNaN(parseRubyFloat("nan")) {
		t.Error("parseRubyFloat(nan)")
	}
}

func TestNamedEncodingRoundTrip(t *testing.T) {
	s := &Str{Bytes: []byte{0x82, 0xa0}, Enc: Named, Name: "Shift_JIS"}
	out := Dump(s)
	back, err := Load(out)
	if err != nil {
		t.Fatalf("Load named: %v", err)
	}
	bs := back.(*Str)
	if bs.Enc != Named || bs.Name != "Shift_JIS" || !bytes.Equal(bs.Bytes, s.Bytes) {
		t.Errorf("named encoding round-trip = %+v", bs)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name  string
		bytes string
	}{
		{"empty", ""},
		{"oneByte", "04"},
		{"badVersion", "05 08 30"},
		{"unknownTag", "04 08 99"},
		{"truncatedInt", "04 08 69"},
		{"truncatedIntBytes", "04 08 69 02 2c"},
		{"truncatedString", "04 08 22 07 68"},
		{"symLinkOOR", "04 08 3b 06"},
		{"objLinkOOR", "04 08 40 06"},
		{"ivarNotString", "04 08 49 30"},                 // I followed by nil
		{"ivarNameNotSymbol", "04 08 49 22 00 06 30 54"}, // ivar name is nil, not symbol
	}
	for _, c := range cases {
		if _, err := Load(hx(c.bytes)); err == nil {
			t.Errorf("Load(%s) = nil error, want error", c.name)
		}
	}
}

func TestLoadEncodingValueNotString(t *testing.T) {
	// I" "" 1ivar :encoding => nil  (encoding value must be a String)
	b := []byte{4, 8, 'I', '"', 0, 6}
	b = append(b, ':', byte(8+5))
	b = append(b, "encoding"...)
	b = append(b, '0') // nil value
	if _, err := Load(b); err == nil {
		t.Error("expected error for non-String encoding value")
	}
}

func TestDumpUnsupportedPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Dump of an unsupported Value did not panic")
		}
	}()
	Dump(unknownValue{})
}

// unknownValue is a Value the encoder does not handle, exercising the default
// branch of writeValue.
type unknownValue struct{}

func (unknownValue) RubyClass() string { return "?" }

func TestErrShortUnwrap(t *testing.T) {
	// A truncated stream yields an error that matches ErrShort via errors.Is.
	_, err := Load(hx("04 08 69"))
	if !errors.Is(err, ErrShort) {
		t.Errorf("truncated Load error = %v, want errors.Is ErrShort", err)
	}
	// marshalError reports and unwraps its wrapped error.
	me := marshalError{ErrShort}
	if me.Error() != ErrShort.Error() || me.Unwrap() != ErrShort {
		t.Errorf("marshalError wrap/unwrap broken: %v", me)
	}
}

// TestObjectLinksGolden covers the shared/cyclic object-link paths with MRI's
// exact bytes, without needing the live interpreter (so CI hits them too).
func TestObjectLinksGolden(t *testing.T) {
	// Shared array: a=[1,2]; [a,a]
	a := &Array{Elems: []Value{NewInt(1), NewInt(2)}}
	if got, want := Dump(&Array{Elems: []Value{a, a}}),
		hx("04 08 5b 07 5b 07 69 06 69 07 40 06"); !bytes.Equal(got, want) {
		t.Errorf("shared array: %x != %x", got, want)
	}

	// Shared string: a="x"; [a,a]
	s := NewString("x")
	if got, want := Dump(&Array{Elems: []Value{s, s}}),
		hx("04 08 5b 07 49 22 06 78 06 3a 06 45 54 40 06"); !bytes.Equal(got, want) {
		t.Errorf("shared string: %x != %x", got, want)
	}

	// Cyclic array: a=[]; a<<a
	cyc := &Array{}
	cyc.Elems = []Value{cyc}
	out := Dump(cyc)
	if want := hx("04 08 5b 06 40 00"); !bytes.Equal(out, want) {
		t.Errorf("cyclic: %x != %x", out, want)
	}
	back, err := Load(out)
	if err != nil {
		t.Fatalf("Load cyclic: %v", err)
	}
	if arr := back.(*Array); len(arr.Elems) != 1 || arr.Elems[0] != arr {
		t.Error("cyclic Load did not reconstruct self-reference")
	}
	shared, _ := Load(hx("04 08 5b 07 5b 07 69 06 69 07 40 06"))
	outer := shared.(*Array)
	if outer.Elems[0] != outer.Elems[1] {
		t.Error("shared Load did not share the inner array identity")
	}
}

func TestRubyClass(t *testing.T) {
	for _, c := range []struct {
		v    Value
		want string
	}{
		{Nil{}, "NilClass"},
		{Bool(true), "TrueClass"},
		{Bool(false), "FalseClass"},
		{NewInt(1), "Integer"},
		{Float(1), "Float"},
		{Symbol("x"), "Symbol"},
		{&Str{}, "String"},
		{&Array{}, "Array"},
		{&Hash{}, "Hash"},
	} {
		if got := c.v.RubyClass(); got != c.want {
			t.Errorf("RubyClass(%T) = %q, want %q", c.v, got, c.want)
		}
	}
}
