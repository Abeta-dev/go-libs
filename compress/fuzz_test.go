// SPDX-License-Identifier: MIT

package compress_test

import (
	"testing"

	"github.com/abeta-dev/go-libs/compress"
)

func FuzzNegotiate(f *testing.F) {
	seeds := []string{
		"",
		"br",
		"gzip",
		"gzip, deflate, br",
		"br;q=1.0, gzip;q=0.8, *;q=0.1",
		"br;q=0, *;q=0.5",
		"*;q=0",
		"x-gzip;q=0.9",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, header string) {
		got := compress.Negotiate(header)
		if got != "" && got != compress.EncodingBrotli && got != compress.EncodingGzip {
			t.Fatalf("Negotiate(%q) returned unexpected encoding %q", header, got)
		}
	})
}
