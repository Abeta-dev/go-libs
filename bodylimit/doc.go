// SPDX-License-Identifier: MIT

// Package bodylimit provides HTTP middleware that enforces maximum request body sizes
// wrapping http.MaxBytesReader to defend services against memory exhaustion and volumetric DoS.
package bodylimit
