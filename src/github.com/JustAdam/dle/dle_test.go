// Copyright 2014, 2015 JustAdam (adambell7@gmail.com).  All rights reserved.
// License: MIT

package main

import (
	"bytes"
	"testing"
)

func TestLogWriterWrite(t *testing.T) {
	token := "XX-XX"
	lw := &LogWriter{
		logline: make(chan []byte),
		token:   token,
	}

	testWrites := []struct {
		s, e []byte
	}{
		{[]byte("this is a test"), []byte(token + "this is a test")},
	}

	for _, v := range testWrites {
		go lw.Write(v.s)
		s := <-lw.logline
		if !bytes.Equal(s, v.e) {
			t.Errorf("Expecting %s, got %s", v.e, s)
		}
	}
}
