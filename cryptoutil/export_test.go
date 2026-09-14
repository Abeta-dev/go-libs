// SPDX-License-Identifier: MIT

package cryptoutil

import (
	"crypto/rand"
	"io"
)

func SetBcryptGenerateFromPassword(f func(password []byte, cost int) ([]byte, error)) {
	bcryptGenerateFromPassword = f
}

func ResetBcryptGenerateFromPassword() {
	bcryptGenerateFromPassword = bcryptGenerateFromPasswordDefault
}

var bcryptGenerateFromPasswordDefault = bcryptGenerateFromPassword

func SetRandReader(r io.Reader) {
	randReader = r
}

func ResetRandReader() {
	randReader = rand.Reader
}
