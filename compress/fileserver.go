// SPDX-License-Identifier: MIT

package compress

import (
	"crypto/sha256"
	"encoding/hex"
	"html"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	defaultIndexFile        = "index.html"
	defaultCSRFMetaName     = "csrf-token"
	defaultHTMLCacheControl = "no-cache"
)

type fileServerConfig struct {
	spaFallback           string
	excludePrefixes       []string
	csrfMetaToken         string
	csrfMetaTokenFn       func(*http.Request) string
	csrfMetaName          string
	immutableCacheControl string
	htmlCacheControl      string
	htmlCacheControlSet   bool
	pathPrefix            string
}

// FileServerOption configures PrecompressedFileServer and PrecompressedFS.
type FileServerOption func(*fileServerConfig)

// WithSPAFallback enables Single-Page Application fallback to indexFile (for example
// "index.html") when a requested route does not match a static file, has no file
// extension, and is not excluded by WithFallbackExcludePrefixes.
func WithSPAFallback(indexFile string) FileServerOption {
	return func(c *fileServerConfig) {
		c.spaFallback = strings.TrimLeft(strings.TrimSpace(indexFile), "/")
	}
}

// WithFallbackExcludePrefixes configures URL path prefixes (such as "/api/" or "/healthz")
// that must return 404 Not Found instead of falling back to the SPA index document.
func WithFallbackExcludePrefixes(prefixes ...string) FileServerOption {
	return func(c *fileServerConfig) {
		for _, p := range prefixes {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				c.excludePrefixes = append(c.excludePrefixes, trimmed)
			}
		}
	}
}

// WithCSRFMetaToken injects a static <meta name="csrf-token" content="..."> tag into
// served HTML documents before </head>. When active, precompressed HTML sidecars are
// bypassed so the runtime token is never stale.
func WithCSRFMetaToken(token string) FileServerOption {
	return func(c *fileServerConfig) {
		c.csrfMetaToken = token
	}
}

// WithCSRFMetaTokenProvider registers a dynamic per-request CSRF token provider
// injected as a <meta> tag into served HTML documents before </head>.
func WithCSRFMetaTokenProvider(fn func(*http.Request) string) FileServerOption {
	return func(c *fileServerConfig) {
		c.csrfMetaTokenFn = fn
	}
}

// WithCSRFMetaName overrides the <meta name="..."> attribute used for CSRF token
// injection. Defaults to "csrf-token".
func WithCSRFMetaName(name string) FileServerOption {
	return func(c *fileServerConfig) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			c.csrfMetaName = trimmed
		}
	}
}

// WithImmutableCacheControl sets the Cache-Control header applied to non-HTML static
// assets (such as fingerprinted .wasm, .js, and .css bundles).
func WithImmutableCacheControl(header string) FileServerOption {
	return func(c *fileServerConfig) {
		c.immutableCacheControl = strings.TrimSpace(header)
	}
}

// WithHTMLCacheControl overrides the Cache-Control header applied to HTML documents.
// Defaults to "no-cache" (or "no-store" when CSRF meta token injection is active).
func WithHTMLCacheControl(header string) FileServerOption {
	return func(c *fileServerConfig) {
		c.htmlCacheControl = strings.TrimSpace(header)
		c.htmlCacheControlSet = true
	}
}

// WithPathPrefix strips the specified URL path prefix before looking up files in
// the underlying filesystem.
func WithPathPrefix(prefix string) FileServerOption {
	return func(c *fileServerConfig) {
		c.pathPrefix = strings.TrimRight(strings.TrimSpace(prefix), "/")
	}
}

// PrecompressedFileServer returns an http.Handler serving static assets from rootDir
// using build-time <file>.br and <file>.gz sidecars negotiated via RFC 9110
// Accept-Encoding quality weights, with optional SPA fallback and HTML <meta> CSRF
// token injection.
func PrecompressedFileServer(rootDir string, opts ...FileServerOption) http.Handler {
	return PrecompressedFS(os.DirFS(rootDir), opts...)
}

// PrecompressedFS returns an http.Handler serving static assets from an fs.FS
// (such as embed.FS or os.DirFS) using build-time <file>.br and <file>.gz sidecars.
func PrecompressedFS(fsys fs.FS, opts ...FileServerOption) http.Handler {
	if fsys == nil {
		return http.NotFoundHandler()
	}

	cfg := fileServerConfig{
		csrfMetaName:     defaultCSRFMetaName,
		htmlCacheControl: defaultHTMLCacheControl,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		relPath, ok := resolveStaticPath(r.URL.Path, cfg.pathPrefix, cfg.spaFallback)
		if !ok {
			http.NotFound(w, r)
			return
		}

		targetPath, found := lookupStaticTarget(fsys, relPath, r.URL.Path, cfg)
		if !found {
			http.NotFound(w, r)
			return
		}

		isHTML := isHTMLFile(targetPath)
		if isHTML {
			if token := cfg.resolveCSRFToken(r); token != "" {
				serveInjectedHTML(w, r, fsys, targetPath, token, cfg)
				return
			}
		}

		serveStaticAsset(w, r, fsys, targetPath, isHTML, cfg)
	})
}

func (c fileServerConfig) resolveCSRFToken(r *http.Request) string {
	if c.csrfMetaTokenFn != nil {
		return c.csrfMetaTokenFn(r)
	}
	return c.csrfMetaToken
}

func resolveStaticPath(rawPath, prefix, spaFallback string) (string, bool) {
	if strings.Contains(rawPath, "\x00") || strings.Contains(rawPath, "\\") {
		return "", false
	}
	for _, seg := range strings.Split(rawPath, "/") {
		if seg == ".." {
			return "", false
		}
	}

	p := rawPath
	if prefix != "" {
		if p != prefix && !strings.HasPrefix(p, prefix+"/") {
			return "", false
		}
		p = strings.TrimPrefix(p, prefix)
	}

	cleaned := path.Clean("/" + p)
	if cleaned == "/" || strings.HasSuffix(p, "/") {
		indexName := defaultIndexFile
		if spaFallback != "" {
			indexName = spaFallback
		}
		cleaned = path.Join(cleaned, indexName)
	}

	rel := strings.TrimLeft(cleaned, "/")
	if !fs.ValidPath(rel) {
		return "", false
	}
	return rel, true
}

func lookupStaticTarget(fsys fs.FS, relPath, rawURLPath string, cfg fileServerConfig) (string, bool) {
	if regularFileExists(fsys, relPath) {
		return relPath, true
	}
	if cfg.spaFallback != "" &&
		path.Ext(relPath) == "" &&
		!isExcludedPrefix(rawURLPath, cfg.excludePrefixes) &&
		regularFileExists(fsys, cfg.spaFallback) {
		return cfg.spaFallback, true
	}
	return "", false
}

func regularFileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}

func isExcludedPrefix(rawPath string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(rawPath, p) {
			return true
		}
	}
	return false
}

func isHTMLFile(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return ext == ".html" || ext == ".htm"
}

func serveInjectedHTML(w http.ResponseWriter, r *http.Request, fsys fs.FS, targetPath, token string, cfg fileServerConfig) {
	raw, err := fs.ReadFile(fsys, targetPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	rendered := injectCSRFMeta(raw, cfg.csrfMetaName, token)
	addVaryAcceptEncoding(w.Header())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	cc := "no-store"
	if cfg.htmlCacheControlSet {
		cc = cfg.htmlCacheControl
	}
	if cc != "" {
		w.Header().Set("Cache-Control", cc)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(rendered)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		// #nosec G705 -- CSRF meta attribute values are escaped with html.EscapeString before injection.
		_, _ = w.Write(rendered)
	}
}

func injectCSRFMeta(raw []byte, metaName, token string) []byte {
	tag := `<meta name="` + html.EscapeString(metaName) + `" content="` + html.EscapeString(token) + `">`
	src := string(raw)
	idx := strings.Index(strings.ToLower(src), "</head>")
	if idx >= 0 {
		var sb strings.Builder
		sb.Grow(len(src) + len(tag))
		sb.WriteString(src[:idx])
		sb.WriteString(tag)
		sb.WriteString(src[idx:])
		return []byte(sb.String())
	}
	return []byte(tag + src)
}

func serveStaticAsset(w http.ResponseWriter, r *http.Request, fsys fs.FS, targetPath string, isHTML bool, cfg fileServerConfig) {
	data, enc, err := readBestEncodedFile(fsys, targetPath, r.Header.Get("Accept-Encoding"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h := w.Header()
	addVaryAcceptEncoding(h)
	h.Set("Content-Type", detectStaticContentType(targetPath, data))
	if enc != "" {
		h.Set("Content-Encoding", enc)
	}
	if isHTML {
		if cfg.htmlCacheControl != "" {
			h.Set("Cache-Control", cfg.htmlCacheControl)
		}
	} else if cfg.immutableCacheControl != "" {
		h.Set("Cache-Control", cfg.immutableCacheControl)
	}

	etag := computeAssetETag(data, enc)
	h.Set("ETag", etag)
	if matchStaticETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	h.Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		// #nosec G705 -- Serving static file bytes from trusted root filesystem.
		_, _ = w.Write(data)
	}
}

func readBestEncodedFile(fsys fs.FS, targetPath, acceptEncoding string) (data []byte, encoding string, err error) {
	brQ, gzipQ := negotiateQuality(acceptEncoding)
	for _, candidate := range buildSidecarOrder(brQ, gzipQ) {
		sidecarPath := targetPath + candidate.ext
		if regularFileExists(fsys, sidecarPath) {
			b, readErr := fs.ReadFile(fsys, sidecarPath)
			if readErr == nil {
				return b, candidate.encoding, nil
			}
		}
	}

	raw, err := fs.ReadFile(fsys, targetPath)
	if err != nil {
		return nil, "", err
	}
	return raw, "", nil
}

type sidecarCandidate struct {
	ext      string
	encoding string
}

func buildSidecarOrder(brQ, gzipQ float64) []sidecarCandidate {
	candidates := make([]sidecarCandidate, 0, 2)
	if brQ >= gzipQ {
		if brQ > 0 {
			candidates = append(candidates, sidecarCandidate{ext: ".br", encoding: EncodingBrotli})
		}
		if gzipQ > 0 {
			candidates = append(candidates, sidecarCandidate{ext: ".gz", encoding: EncodingGzip})
		}
		return candidates
	}
	if gzipQ > 0 {
		candidates = append(candidates, sidecarCandidate{ext: ".gz", encoding: EncodingGzip})
	}
	if brQ > 0 {
		candidates = append(candidates, sidecarCandidate{ext: ".br", encoding: EncodingBrotli})
	}
	return candidates
}

func detectStaticContentType(targetPath string, sample []byte) string {
	ext := strings.ToLower(path.Ext(targetPath))
	switch ext {
	case ".wasm":
		return "application/wasm"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return http.DetectContentType(sample)
}

func computeAssetETag(data []byte, encoding string) string {
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:12])
	if encoding != "" {
		return `"` + digest + "-" + encoding + `"`
	}
	return `"` + digest + `"`
}

func matchStaticETag(ifNoneMatch, current string) bool {
	if strings.TrimSpace(ifNoneMatch) == "" {
		return false
	}
	normCurrent := strings.TrimPrefix(current, "W/")
	for _, token := range strings.Split(ifNoneMatch, ",") {
		t := strings.TrimSpace(token)
		if t == "*" || strings.TrimPrefix(t, "W/") == normCurrent {
			return true
		}
	}
	return false
}
