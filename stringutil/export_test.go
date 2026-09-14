// SPDX-License-Identifier: MIT

package stringutil

import (
	"crypto/rand"
	"io"
)

func SetRandReader(r io.Reader) {
	randReader = r
}

func ResetRandReader() {
	randReader = rand.Reader
}
