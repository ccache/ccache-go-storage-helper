// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package storagehelper

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWriteMsgEncodesValidUTF8WithinByteLimit(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{name: "short message", msg: "hello", want: "hello"},
		{name: "truncates before partial rune", msg: strings.Repeat("a", 254) + "€", want: strings.Repeat("a", 254)},
		{name: "replaces invalid UTF-8", msg: string([]byte{'a', 0xff, 'b'}), want: "a�b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			if err := writeMsg(&buffer, tt.msg); err != nil {
				t.Fatalf("writeMsg: %v", err)
			}
			encoded := buffer.Bytes()
			if len(encoded) == 0 {
				t.Fatal("empty encoded message")
			}
			if int(encoded[0]) != len(encoded)-1 {
				t.Errorf("length = %d, want %d", encoded[0], len(encoded)-1)
			}
			got := string(encoded[1:])
			if !utf8.ValidString(got) {
				t.Errorf("message is not valid UTF-8: %x", got)
			}
			if got != tt.want {
				t.Errorf("message = %q, want %q", got, tt.want)
			}
		})
	}
}
