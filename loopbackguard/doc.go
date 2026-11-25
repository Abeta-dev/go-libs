// SPDX-License-Identifier: MIT

// Package loopbackguard provides local-first loopback address validation,
// DNS-rebinding Host allowlisting, Sec-Fetch-Site / Origin Same-Origin checks,
// and constant-time per-boot ephemeral CSRF token verification for localhost
// BFFs and desktop Go services.
package loopbackguard
