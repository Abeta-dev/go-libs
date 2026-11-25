// SPDX-License-Identifier: MIT

package ginmw

import (
	"io/fs"
	"net/http"

	"github.com/abeta-dev/go-libs/compress"
	"github.com/gin-gonic/gin"
)

type ginCompressWriter struct {
	gin.ResponseWriter
	cw http.ResponseWriter
}

func (w *ginCompressWriter) Write(b []byte) (int, error) {
	return w.cw.Write(b)
}

func (w *ginCompressWriter) WriteString(s string) (int, error) {
	return w.cw.Write([]byte(s))
}

func (w *ginCompressWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.cw.WriteHeader(code)
}

func (w *ginCompressWriter) Flush() {
	if f, ok := w.cw.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *ginCompressWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Compress returns a Gin middleware that negotiates Brotli ("br") or Gzip ("gzip")
// response compression and optionally decompresses incoming compressed request bodies.
func Compress(cfg ...compress.Config) gin.HandlerFunc {
	mw := compress.Middleware(cfg...)
	return func(c *gin.Context) {
		origWriter := c.Writer
		called := false
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			c.Request = r
			c.Writer = &ginCompressWriter{
				ResponseWriter: origWriter,
				cw:             w,
			}
			defer func() {
				c.Writer = origWriter
			}()
			c.Next()
		}))
		handler.ServeHTTP(origWriter, c.Request)
		if !called {
			c.Abort()
		}
	}
}

// PrecompressedFileServer returns a Gin handler serving static files from rootDir
// using precompressed .br / .gz sidecars with optional SPA fallback and CSRF meta injection.
func PrecompressedFileServer(rootDir string, opts ...compress.FileServerOption) gin.HandlerFunc {
	return gin.WrapH(compress.PrecompressedFileServer(rootDir, opts...))
}

// PrecompressedFS returns a Gin handler serving static files from an fs.FS
// using precompressed .br / .gz sidecars with optional SPA fallback and CSRF meta injection.
func PrecompressedFS(fsys fs.FS, opts ...compress.FileServerOption) gin.HandlerFunc {
	return gin.WrapH(compress.PrecompressedFS(fsys, opts...))
}
