// Copyright 2015 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

package jobs

import (
	"reflect"
	"testing"
)

// NOTE:
// I know this code isn't very dry. Unfortunately, this is due to a restriction
// in reflect and gob packages. We can't have a general test case or test function
// that treats different types as interfaces, because then gob doesn't know what type
// to attempt to decode into. So we have to test the different types manually.

func TestConvertInt(t *testing.T) {
	v := int(7)
	// Encode v to a slice of bytes
	reply, err := encode(v)
	if err != nil {
		t.Errorf("Unexpected error in encode: %s", err.Error())
	}
	// Decode reply and write results to the holder
	holder := int(0)
	if err := decode(reply, &holder); err != nil {
		t.Errorf("Unexpected error in decode: %s", err.Error())
	}
	// Now the holder and the original should be equal. If they're not,
	// there was a problem
	expectEncodeDecodeEquals(t, v, holder)
}

func TestConvertString(t *testing.T) {
	v := "test"
	// Encode v to a slice of bytes
	reply, err := encode(v)
	if err != nil {
		t.Errorf("Unexpected error in encode: %s", err.Error())
	}
	// Decode reply and write results to the holder
	holder := ""
	if err := decode(reply, &holder); err != nil {
		t.Errorf("Unexpected error in decode: %s", err.Error())
	}
	// Now the holder and the original should be equal. If they're not,
	// there was a problem
	expectEncodeDecodeEquals(t, v, holder)
}

func TestConvertBool(t *testing.T) {
	v := true
	// Encode v to a slice of bytes
	reply, err := encode(v)
	if err != nil {
		t.Errorf("Unexpected error in encode: %s", err.Error())
	}
	// Decode reply and write results to the holder
	holder := false
	if err := decode(reply, &holder); err != nil {
		t.Errorf("Unexpected error in decode: %s", err.Error())
	}
	// Now the holder and the original should be equal. If they're not,
	// there was a problem
	expectEncodeDecodeEquals(t, v, holder)
}

func TestConvertStruct(t *testing.T) {
	v := struct {
		Name string
		Age  int
	}{"test person", 23}
	// Encode v to a slice of bytes
	reply, err := encode(v)
	if err != nil {
		t.Errorf("Unexpected error in encode: %s", err.Error())
	}
	// Decode reply and write results to the holder
	holder := struct {
		Name string
		Age  int
	}{}
	if err := decode(reply, &holder); err != nil {
		t.Errorf("Unexpected error in decode: %s", err.Error())
	}
	// Now the holder and the original should be equal. If they're not,
	// there was a problem
	expectEncodeDecodeEquals(t, v, holder)
}

func TestConvertSlice(t *testing.T) {
	v := []string{"a", "b", "c"}
	// Encode v to a slice of bytes
	reply, err := encode(v)
	if err != nil {
		t.Errorf("Unexpected error in encode: %s", err.Error())
	}
	// Decode reply and write results to the holder
	holder := []string{}
	if err := decode(reply, &holder); err != nil {
		t.Errorf("Unexpected error in decode: %s", err.Error())
	}
	// Now the holder and the original should be equal. If they're not,
	// there was a problem
	expectEncodeDecodeEquals(t, v, holder)
}

func expectEncodeDecodeEquals(t *testing.T, expected, got interface{}) {
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("Error encoding/decoding type %T. Expected %v but got %v.", expected, expected, got)
	}
}
