// SPDX-License-Identifier: MIT

// Package compress provides production-hardened HTTP response compression
// (Brotli "br" and Gzip "gzip") with RFC 9110 Accept-Encoding q-value
// negotiation, pooled writers, threshold buffering, and bounded request body
// decompression with zip-bomb protection.
package compress

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

// Supported HTTP Content-Encoding tokens.
const (
	EncodingBrotli   = "br"
	EncodingGzip     = "gzip"
	EncodingIdentity = "identity"
)

const (
	defaultMinSize              = 1024
	defaultMaxDecompressedBytes = 10 << 20 // 10 MiB
)

// Config controls HTTP response compression and request body decompression.
type Config struct {
	// MinSize is the minimum response payload size in bytes required before
	// compression is applied. Payloads smaller than MinSize are sent uncompressed.
	// Defaults to 1024 bytes when <= 0.
	MinSize int

	// GzipLevel is the compress/gzip compression level.
	// Defaults to gzip.DefaultCompression when outside [gzip.HuffmanOnly, gzip.BestCompression].
	GzipLevel int

	// BrotliLevel is the Brotli compression quality level (0..11).
	// Defaults to brotli.DefaultCompression when outside [brotli.BestSpeed, brotli.BestCompression].
	BrotliLevel int

	// ContentTypes lists compressible MIME types or prefix patterns (such as "text/").
	// Defaults to DefaultConfig().ContentTypes when empty.
	ContentTypes []string

	// DecompressRequest enables automatic decompression of incoming requests
	// carrying Content-Encoding: br or Content-Encoding: gzip.
	DecompressRequest bool

	// MaxDecompressedBytes caps decompressed request payload size when
	// DecompressRequest is true to defend against zip-bomb attacks.
	// Exceeding this limit returns 413 Request Entity Too Large.
	// Defaults to 10 MiB (10 << 20) when <= 0.
	MaxDecompressedBytes int64
}

// DefaultConfig returns the recommended production compression configuration.
// Note: Server-Sent Events ("text/event-stream") are always excluded from
// buffering and compression even though "text/" is listed.
func DefaultConfig() Config {
	return Config{
		MinSize:     defaultMinSize,
		GzipLevel:   gzip.DefaultCompression,
		BrotliLevel: brotli.DefaultCompression,
		ContentTypes: []string{
			"application/json",
			"application/problem+json",
			"application/javascript",
			"application/xml",
			"application/xhtml+xml",
			"image/svg+xml",
			"text/",
		},
		DecompressRequest:    false,
		MaxDecompressedBytes: defaultMaxDecompressedBytes,
	}
}

func normalizeConfig(cfg ...Config) Config {
	c := DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
		if c.MinSize <= 0 {
			c.MinSize = defaultMinSize
		}
		if c.GzipLevel < gzip.HuffmanOnly || c.GzipLevel > gzip.BestCompression {
			c.GzipLevel = gzip.DefaultCompression
		}
		if c.BrotliLevel < brotli.BestSpeed || c.BrotliLevel > brotli.BestCompression {
			c.BrotliLevel = brotli.DefaultCompression
		}
		if len(c.ContentTypes) == 0 {
			c.ContentTypes = DefaultConfig().ContentTypes
		}
		if c.MaxDecompressedBytes <= 0 {
			c.MaxDecompressedBytes = defaultMaxDecompressedBytes
		}
	}
	return c
}

// Negotiate parses an RFC 9110 Accept-Encoding header with optional q-value weights
// and returns the preferred supported encoding ("br", "gzip", or "" if neither is
// acceptable). When both Brotli and Gzip have equal positive quality weights,
// Brotli ("br") is preferred.
func Negotiate(acceptEncoding string) string {
	brQ, gzipQ := negotiateQuality(acceptEncoding)
	return selectPreferredEncoding(brQ, gzipQ)
}

func negotiateQuality(acceptEncoding string) (brQ, gzipQ float64) {
	if strings.TrimSpace(acceptEncoding) == "" {
		return -1.0, -1.0
	}

	brQ, gzipQ = -1.0, -1.0
	starQ := -1.0

	for _, part := range strings.Split(acceptEncoding, ",") {
		token, q, ok := parseEncodingDirective(part)
		if !ok {
			continue
		}
		switch token {
		case EncodingBrotli:
			brQ = q
		case EncodingGzip, "x-gzip":
			gzipQ = q
		case "*":
			starQ = q
		}
	}

	if brQ < 0 && starQ >= 0 {
		brQ = starQ
	}
	if gzipQ < 0 && starQ >= 0 {
		gzipQ = starQ
	}
	return brQ, gzipQ
}

func parseEncodingDirective(part string) (token string, q float64, ok bool) {
	part = strings.TrimSpace(part)
	if part == "" {
		return "", 0, false
	}

	segments := strings.Split(part, ";")
	token = strings.ToLower(strings.TrimSpace(segments[0]))
	q = 1.0

	for _, param := range segments[1:] {
		p := strings.ToLower(strings.TrimSpace(param))
		if strings.HasPrefix(p, "q=") {
			val, err := strconv.ParseFloat(p[2:], 64)
			if err != nil || math.IsNaN(val) || val < 0 || val > 1 {
				return "", 0, false
			}
			q = val
		}
	}
	return token, q, true
}

func selectPreferredEncoding(brQ, gzipQ float64) string {
	if brQ <= 0 && gzipQ <= 0 {
		return ""
	}
	if brQ >= gzipQ {
		return EncodingBrotli
	}
	return EncodingGzip
}

// Middleware returns a net/http middleware that negotiates Brotli ("br") or
// Gzip ("gzip") response compression and optionally decompresses incoming
// compressed request bodies.
func Middleware(cfg ...Config) func(http.Handler) http.Handler {
	c := normalizeConfig(cfg...)

	gzipPool := &sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, c.GzipLevel)
			return w
		},
	}
	brotliPool := &sync.Pool{
		New: func() any {
			return brotli.NewWriterLevel(io.Discard, c.BrotliLevel)
		},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c.DecompressRequest && r.Body != nil {
				enc := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding")))
				if enc == EncodingBrotli || enc == EncodingGzip {
					data, status, err := decompressRequestBody(r, enc, c.MaxDecompressedBytes)
					if err != nil {
						http.Error(w, err.Error(), status)
						return
					}
					r.Body = io.NopCloser(bytes.NewReader(data))
					r.ContentLength = int64(len(data))
					r.Header.Del("Content-Encoding")
					r.Header.Del("Content-Length")
				}
			}

			addVaryAcceptEncoding(w.Header())

			encoding := Negotiate(r.Header.Get("Accept-Encoding"))
			if encoding == "" || r.Method == http.MethodHead || r.Header.Get("Upgrade") != "" {
				next.ServeHTTP(w, r)
				return
			}

			cw := &compressResponseWriter{
				ResponseWriter: w,
				cfg:            c,
				encoding:       encoding,
				statusCode:     http.StatusOK,
				gzipPool:       gzipPool,
				brotliPool:     brotliPool,
			}
			defer cw.finish()

			next.ServeHTTP(cw, r)
		})
	}
}

func decompressRequestBody(r *http.Request, enc string, maxBytes int64) (decompressed []byte, statusCode int, readErr error) {
	defer func() { _ = r.Body.Close() }()

	var reader io.Reader
	if enc == EncodingBrotli {
		reader = brotli.NewReader(r.Body)
	} else {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	}

	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	if int64(len(data)) > maxBytes {
		return nil, http.StatusRequestEntityTooLarge, errors.New("compress: decompressed request body exceeds limit")
	}
	return data, http.StatusOK, nil
}

func addVaryAcceptEncoding(h http.Header) {
	for _, v := range h.Values("Vary") {
		for _, token := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(token), "Accept-Encoding") {
				return
			}
		}
	}
	h.Add("Vary", "Accept-Encoding")
}

func hasNoTransform(values []string) bool {
	for _, v := range values {
		for _, directive := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(directive), "no-transform") {
				return true
			}
		}
	}
	return false
}

func isEventStream(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	return strings.EqualFold(strings.TrimSpace(mediaType), "text/event-stream")
}

func isCompressible(contentType string, allowed []string) bool {
	if isEventStream(contentType) {
		return false
	}
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	for _, pattern := range allowed {
		p := strings.ToLower(strings.TrimSpace(pattern))
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(mediaType, p) {
				return true
			}
		} else if mediaType == p {
			return true
		}
	}
	return false
}

type compressWriter interface {
	io.WriteCloser
	Flush() error
	Reset(w io.Writer)
}

type compressResponseWriter struct {
	http.ResponseWriter
	cfg           Config
	encoding      string
	statusCode    int
	headerWritten bool
	decided       bool
	buf           []byte
	zw            compressWriter
	gzipPool      *sync.Pool
	brotliPool    *sync.Pool
}

func (cw *compressResponseWriter) WriteHeader(code int) {
	if !cw.headerWritten {
		cw.statusCode = code
		cw.headerWritten = true
		if !cw.decided && isEventStream(cw.Header().Get("Content-Type")) {
			_ = cw.start()
		}
	}
}

func (cw *compressResponseWriter) Write(p []byte) (int, error) {
	if !cw.headerWritten {
		cw.WriteHeader(http.StatusOK)
	}
	if !cw.decided {
		cw.buf = append(cw.buf, p...)
		if len(cw.buf) < cw.cfg.MinSize {
			return len(p), nil
		}
		err := cw.start()
		return len(p), err
	}
	if cw.zw != nil {
		return cw.zw.Write(p)
	}
	return cw.ResponseWriter.Write(p)
}

func (cw *compressResponseWriter) Flush() {
	if !cw.decided {
		_ = cw.start()
	}
	if cw.zw != nil {
		_ = cw.zw.Flush()
	}
	if f, ok := cw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (cw *compressResponseWriter) Unwrap() http.ResponseWriter {
	return cw.ResponseWriter
}

func (cw *compressResponseWriter) start() error {
	cw.decided = true
	if len(cw.buf) > 0 && cw.Header().Get("Content-Type") == "" {
		cw.Header().Set("Content-Type", http.DetectContentType(cw.buf))
	}
	if cw.shouldCompress() {
		cw.Header().Del("Content-Length")
		cw.Header().Set("Content-Encoding", cw.encoding)
		addVaryAcceptEncoding(cw.Header())
		cw.ResponseWriter.WriteHeader(cw.statusCode)
		cw.zw = cw.acquireWriter()
		_, err := cw.zw.Write(cw.buf)
		cw.buf = nil
		return err
	}
	if cw.headerWritten {
		cw.ResponseWriter.WriteHeader(cw.statusCode)
	}
	if len(cw.buf) > 0 {
		_, err := cw.ResponseWriter.Write(cw.buf)
		cw.buf = nil
		return err
	}
	return nil
}

func (cw *compressResponseWriter) shouldCompress() bool {
	if cw.statusCode < 200 || cw.statusCode == http.StatusNoContent || cw.statusCode == http.StatusNotModified {
		return false
	}
	if len(cw.buf) < cw.cfg.MinSize {
		return false
	}
	if cw.Header().Get("Content-Encoding") != "" {
		return false
	}
	if hasNoTransform(cw.Header().Values("Cache-Control")) {
		return false
	}
	return isCompressible(cw.Header().Get("Content-Type"), cw.cfg.ContentTypes)
}

func (cw *compressResponseWriter) acquireWriter() compressWriter {
	if cw.encoding == EncodingBrotli {
		w := cw.brotliPool.Get().(*brotli.Writer)
		w.Reset(cw.ResponseWriter)
		return w
	}
	w := cw.gzipPool.Get().(*gzip.Writer)
	w.Reset(cw.ResponseWriter)
	return w
}

func (cw *compressResponseWriter) releaseWriter() {
	cw.zw.Reset(io.Discard)
	if cw.encoding == EncodingBrotli {
		cw.brotliPool.Put(cw.zw)
		return
	}
	cw.gzipPool.Put(cw.zw)
}

func (cw *compressResponseWriter) finish() {
	if !cw.decided {
		_ = cw.start()
	}
	if cw.zw != nil {
		_ = cw.zw.Close()
		cw.releaseWriter()
		cw.zw = nil
	}
}
