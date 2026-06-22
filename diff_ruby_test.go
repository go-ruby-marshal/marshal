// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-marshal/marshal authors

package marshal

import (
	"bytes"
	"math/big"
	"os"
	"os/exec"
	"testing"
)

// rubyBin is the reference MRI interpreter used for live byte-for-byte
// differential checks. To keep these checks pinned to a known Ruby (the golden
// byte vectors in marshal_test.go already encode MRI 4.0.5's output and provide
// full coverage on their own), it returns the oracle only at its known path or
// via the MARSHAL_RUBY override — not any `ruby` on PATH. Differential tests
// skip when it is absent, so CI stays green regardless of the runner's Ruby.
func rubyBin() string {
	if p := os.Getenv("MARSHAL_RUBY"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return ""
	}
	const oracle = "/opt/homebrew/opt/ruby/bin/ruby"
	if _, err := os.Stat(oracle); err == nil {
		return oracle
	}
	return ""
}

// rubyDump returns MRI's Marshal.dump(<expr>) bytes.
func rubyDump(t *testing.T, ruby, expr string) []byte {
	t.Helper()
	out, err := exec.Command(ruby, "-e", "STDOUT.binmode; STDOUT.write(Marshal.dump("+expr+"))").Output()
	if err != nil {
		t.Fatalf("ruby dump %q: %v", expr, err)
	}
	return out
}

// rubyDumpProg runs a full Ruby program that must assign the object to `obj`;
// it returns Marshal.dump(obj). Used for multi-statement setups (shared/cyclic).
func rubyDumpProg(t *testing.T, ruby, prog string) []byte {
	t.Helper()
	out, err := exec.Command(ruby, "-e", prog+"; STDOUT.binmode; STDOUT.write(Marshal.dump(obj))").Output()
	if err != nil {
		t.Fatalf("ruby prog %q: %v", prog, err)
	}
	return out
}

// rubyInspect returns MRI's Marshal.load of data, then .inspect — used to prove
// our Dump output loads back to the expected value in real Ruby.
func rubyLoadInspect(t *testing.T, ruby string, data []byte) string {
	t.Helper()
	cmd := exec.Command(ruby, "-e", "STDIN.binmode; print Marshal.load(STDIN.read).inspect")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ruby load: %v", err)
	}
	return string(out)
}

func TestDumpMatchesMRI(t *testing.T) {
	ruby := rubyBin()
	if ruby == "" {
		t.Skip("ruby oracle not available")
	}
	big70 := new(big.Int).Lsh(big.NewInt(1), 70)
	cases := []struct {
		expr string
		val  Value
	}{
		{"nil", Nil{}},
		{"true", Bool(true)},
		{"false", Bool(false)},
		{"0", NewInt(0)},
		{"1", NewInt(1)},
		{"122", NewInt(122)},
		{"123", NewInt(123)},
		{"300", NewInt(300)},
		{"70000", NewInt(70000)},
		{"-1", NewInt(-1)},
		{"-123", NewInt(-123)},
		{"-300", NewInt(-300)},
		{"1073741823", NewInt(1073741823)},   // max Fixnum-marshal
		{"-1073741824", NewInt(-1073741824)}, // min Fixnum-marshal
		{"1073741824", NewInt(1073741824)},   // first Bignum
		{"-1073741825", NewInt(-1073741825)},
		{"2**70", Int{big70}},
		{"-(2**70)", Int{new(big.Int).Neg(big70)}},
		{"1.5", Float(1.5)},
		{"1.0", Float(1.0)},
		{"100.0", Float(100.0)},
		{"0.1", Float(0.1)},
		{"-2.5", Float(-2.5)},
		{"1e20", Float(1e20)},
		{"1e-10", Float(1e-10)},
		{"123456789.0", Float(123456789.0)},
		{"-0.0", Float(negZero())},
		{"(1.0/0.0)", Float(posInf())},
		{"(-1.0/0.0)", Float(negInf())},
		{":abc", Symbol("abc")},
		{`"hi"`, NewString("hi")},
		{`""`, NewString("")},
		{`"café"`, NewString("café")},
		{`"hi".b`, &Str{Bytes: []byte("hi"), Enc: ASCII8BIT}},
		{`"hi".force_encoding("US-ASCII")`, &Str{Bytes: []byte("hi"), Enc: USASCII}},
		{"[1,2,3]", &Array{Elems: []Value{NewInt(1), NewInt(2), NewInt(3)}}},
		{"[]", &Array{}},
		{"[:a, :a]", &Array{Elems: []Value{Symbol("a"), Symbol("a")}}},
		{"{1=>2}", &Hash{Keys: []Value{NewInt(1)}, Vals: []Value{NewInt(2)}}},
		{"{}", &Hash{}},
		{"Hash.new(0)", &Hash{Default: NewInt(0)}},
		{`{"k"=>[1,nil,true]}`, &Hash{Keys: []Value{NewString("k")},
			Vals: []Value{&Array{Elems: []Value{NewInt(1), Nil{}, Bool(true)}}}}},
	}
	for _, c := range cases {
		want := rubyDump(t, ruby, c.expr)
		got := Dump(c.val)
		if !bytes.Equal(got, want) {
			t.Errorf("Dump(%s):\n got  %x\n want %x", c.expr, got, want)
			continue
		}
		// Round-trip: our Load of our bytes reproduces an equal value.
		back, err := Load(got)
		if err != nil {
			t.Errorf("Load(Dump(%s)): %v", c.expr, err)
			continue
		}
		if rb := Dump(back); !bytes.Equal(rb, want) {
			t.Errorf("round-trip(%s): re-dump %x != %x", c.expr, rb, want)
		}
	}
}

// TestSharedAndCyclic checks object links against MRI for a shared array, a
// shared string, and a self-referential (cyclic) array.
func TestSharedAndCyclic(t *testing.T) {
	ruby := rubyBin()
	if ruby == "" {
		t.Skip("ruby oracle not available")
	}

	shared := &Array{Elems: []Value{NewInt(1), NewInt(2)}}
	if got, want := Dump(&Array{Elems: []Value{shared, shared}}), rubyDumpProg(t, ruby, "a=[1,2]; obj=[a,a]"); !bytes.Equal(got, want) {
		t.Errorf("shared array: got %x want %x", got, want)
	}

	s := NewString("x")
	if got, want := Dump(&Array{Elems: []Value{s, s}}), rubyDumpProg(t, ruby, `a="x"; obj=[a,a]`); !bytes.Equal(got, want) {
		t.Errorf("shared string: got %x want %x", got, want)
	}

	cyc := &Array{}
	cyc.Elems = []Value{cyc}
	got := Dump(cyc)
	if want := rubyDumpProg(t, ruby, "obj=[]; obj<<obj"); !bytes.Equal(got, want) {
		t.Errorf("cyclic array: got %x want %x", got, want)
	}
	// Real MRI must accept and round-trip our cyclic dump.
	if ins := rubyLoadInspect(t, ruby, got); ins != "[[...]]" {
		t.Errorf("cyclic load inspect = %q, want %q", ins, "[[...]]")
	}
	// Our own Load must rebuild the cycle (first element is the array itself).
	back, err := Load(got)
	if err != nil {
		t.Fatalf("Load cyclic: %v", err)
	}
	arr := back.(*Array)
	if len(arr.Elems) != 1 || arr.Elems[0] != arr {
		t.Errorf("cyclic Load did not reconstruct self-reference")
	}
}

func negZero() float64 { var z float64; return -z }
func posInf() float64  { return 1.0 / zero() }
func negInf() float64  { return -1.0 / zero() }
func zero() float64    { return 0 }
