// SPDX-License-Identifier: MIT

package etag

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Standard HTTP and version-gating header names.
const (
	HeaderETag             = "ETag"
	HeaderIfNoneMatch      = "If-None-Match"
	HeaderAppVersion       = "X-App-Version"
	HeaderPageVersion      = "X-Page-Version"
	HeaderClientAppVersion = "X-Client-App-Version"

	// DefaultCacheControl is applied to ETag-revalidating responses unless overridden.
	DefaultCacheControl = "private, no-cache"
	// DefaultMaxBufferBytes caps response buffering to prevent OOM on large streams (4 MiB).
	DefaultMaxBufferBytes = 4 << 20
)

type config struct {
	appVersion          string
	pageVersion         string
	ignoreJSONKeys      map[string]struct{}
	cacheControl        string
	cacheControlSet     bool
	weak                bool
	clientVersionHeader string
	maxBufferBytes      int
	methods             map[string]struct{}
}

// Option configures ETag computation, Middleware, and WriteVersionedJSON.
type Option func(*config)

// WithAppVersion sets the application build version stamped in X-App-Version,
// mixed into the ETag digest, and compared against X-Client-App-Version for auto-busting.
func WithAppVersion(version string) Option {
	return func(c *config) {
		c.appVersion = strings.TrimSpace(version)
	}
}

// WithPageVersion sets an explicit X-Page-Version header value mixed into the ETag digest.
// When unset and not provided by the handler, X-Page-Version defaults to the canonical content digest.
func WithPageVersion(version string) Option {
	return func(c *config) {
		c.pageVersion = strings.TrimSpace(version)
	}
}

// WithIgnoreJSONKeys configures top-level and nested JSON object keys (such as
// "cached_at_iso" or "generated_at_iso") to strip when computing the canonical ETag hash.
// The HTTP response body itself is returned verbatim without stripping.
func WithIgnoreJSONKeys(keys ...string) Option {
	return func(c *config) {
		if c.ignoreJSONKeys == nil {
			c.ignoreJSONKeys = make(map[string]struct{}, len(keys))
		}
		for _, k := range keys {
			if trimmed := strings.TrimSpace(k); trimmed != "" {
				c.ignoreJSONKeys[trimmed] = struct{}{}
			}
		}
	}
}

// WithCacheControl overrides the Cache-Control header applied to ETag responses.
// Pass "" to leave Cache-Control untouched. Defaults to "private, no-cache".
func WithCacheControl(value string) Option {
	return func(c *config) {
		c.cacheControl = strings.TrimSpace(value)
		c.cacheControlSet = true
	}
}

// WithWeak controls whether emitted ETags use the weak W/"..." validator syntax.
func WithWeak(weak bool) Option {
	return func(c *config) {
		c.weak = weak
	}
}

// WithClientVersionHeader overrides the incoming request header inspected for
// client app version mismatch auto-busting. Defaults to "X-Client-App-Version".
func WithClientVersionHeader(header string) Option {
	return func(c *config) {
		if trimmed := strings.TrimSpace(header); trimmed != "" {
			c.clientVersionHeader = trimmed
		}
	}
}

// WithMaxBufferBytes overrides the maximum buffered response size in bytes.
// Responses larger than this threshold bypass ETag buffering and stream directly.
// Defaults to DefaultMaxBufferBytes (4 MiB).
func WithMaxBufferBytes(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxBufferBytes = n
		}
	}
}

// WithMethods overrides the HTTP methods eligible for ETag evaluation.
// Defaults to GET and HEAD.
func WithMethods(methods ...string) Option {
	return func(c *config) {
		m := make(map[string]struct{}, len(methods))
		for _, method := range methods {
			if trimmed := strings.ToUpper(strings.TrimSpace(method)); trimmed != "" {
				m[trimmed] = struct{}{}
			}
		}
		if len(m) > 0 {
			c.methods = m
		}
	}
}

func buildConfig(opts ...Option) config {
	cfg := config{
		cacheControl:        DefaultCacheControl,
		clientVersionHeader: HeaderClientAppVersion,
		maxBufferBytes:      DefaultMaxBufferBytes,
		methods: map[string]struct{}{
			http.MethodGet:  {},
			http.MethodHead: {},
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

// Compute returns a deterministic RFC 9110 ETag for body using the supplied options.
func Compute(body []byte, opts ...Option) string {
	cfg := buildConfig(opts...)
	tag, _ := computeTagAndDigest(body, "application/json", cfg, cfg.pageVersion)
	return tag
}

func computeTagAndDigest(body []byte, contentType string, cfg config, pageVersion string) (etagVal, shortDigest string) {
	canonical := body
	if len(cfg.ignoreJSONKeys) > 0 && isJSONContentType(contentType) {
		if stripped, ok := stripJSONKeys(body, cfg.ignoreJSONKeys); ok {
			canonical = stripped
		}
	}

	h := sha256.New()
	if cfg.appVersion != "" {
		_, _ = h.Write([]byte("app:" + cfg.appVersion + "\x00"))
	}
	if pageVersion != "" {
		_, _ = h.Write([]byte("page:" + pageVersion + "\x00"))
	}
	_, _ = h.Write(canonical)
	sum := h.Sum(nil)
	digest := hex.EncodeToString(sum[:16])
	shortDigest = digest[:16]
	if cfg.weak {
		return `W/"` + digest + `"`, shortDigest
	}
	return `"` + digest + `"`, shortDigest
}

func isJSONContentType(ct string) bool {
	if strings.TrimSpace(ct) == "" {
		return true
	}
	media, _, _ := strings.Cut(ct, ";")
	media = strings.ToLower(strings.TrimSpace(media))
	return media == "application/json" || strings.HasSuffix(media, "+json")
}

func stripJSONKeys(body []byte, ignored map[string]struct{}) ([]byte, bool) {
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, false
	}
	cleaned := pruneJSONValue(parsed, ignored)
	out, _ := json.Marshal(cleaned)
	return out, true
}

func pruneJSONValue(v any, ignored map[string]struct{}) any {
	switch val := v.(type) {
	case map[string]any:
		res := make(map[string]any, len(val))
		for k, child := range val {
			if _, skip := ignored[k]; skip {
				continue
			}
			res[k] = pruneJSONValue(child, ignored)
		}
		return res
	case []any:
		res := make([]any, len(val))
		for i, item := range val {
			res[i] = pruneJSONValue(item, ignored)
		}
		return res
	default:
		return v
	}
}

// Match performs RFC 9110 Section 13.1.2 weak ETag comparison against an
// If-None-Match header value (supporting "*" and comma-separated ETag lists).
func Match(ifNoneMatch, currentETag string) bool {
	if strings.TrimSpace(ifNoneMatch) == "" || strings.TrimSpace(currentETag) == "" {
		return false
	}
	target := normalizeWeakTag(currentETag)
	for _, part := range strings.Split(ifNoneMatch, ",") {
		candidate := strings.TrimSpace(part)
		if candidate == "*" || normalizeWeakTag(candidate) == target {
			return true
		}
	}
	return false
}

func normalizeWeakTag(tag string) string {
	t := strings.TrimSpace(tag)
	if strings.HasPrefix(t, "W/") || strings.HasPrefix(t, "w/") {
		return t[2:]
	}
	return t
}

func isClientVersionMismatch(r *http.Request, cfg config) bool {
	if cfg.appVersion == "" {
		return false
	}
	clientVer := strings.TrimSpace(r.Header.Get(cfg.clientVersionHeader))
	return clientVer != "" && clientVer != cfg.appVersion
}

func addVaryHeader(h http.Header, token string) {
	for _, existing := range h.Values("Vary") {
		for _, part := range strings.Split(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return
			}
		}
	}
	h.Add("Vary", token)
}

func hasNoStore(values []string) bool {
	for _, v := range values {
		for _, directive := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
				return true
			}
		}
	}
	return false
}

func isEventStream(contentType string) bool {
	media, _, _ := strings.Cut(contentType, ";")
	return strings.EqualFold(strings.TrimSpace(media), "text/event-stream")
}

// WriteVersionedJSON marshals payload as JSON, computes its version-gated ETag
// (honoring WithAppVersion, WithPageVersion, WithIgnoreJSONKeys, and
// X-Client-App-Version auto-bust), and writes either 304 Not Modified (0 bytes)
// or the JSON payload with status.
func WriteVersionedJSON(w http.ResponseWriter, r *http.Request, status int, payload any, opts ...Option) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cfg := buildConfig(opts...)
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")

	pageVer := strings.TrimSpace(h.Get(HeaderPageVersion))
	if pageVer == "" {
		pageVer = cfg.pageVersion
	}
	etagVal, shortDigest := computeTagAndDigest(raw, "application/json", cfg, pageVer)
	if pageVer == "" {
		pageVer = shortDigest
	}

	applyVersionHeaders(h, cfg, etagVal, pageVer)

	if status == http.StatusOK && !isClientVersionMismatch(r, cfg) && Match(r.Header.Get(HeaderIfNoneMatch), etagVal) {
		h.Del("Content-Length")
		h.Del("Content-Type")
		w.WriteHeader(http.StatusNotModified)
		return nil
	}

	h.Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		// #nosec G705 -- Marshaled application JSON payload written with application/json Content-Type.
		_, _ = w.Write(raw)
	}
	return nil
}

func applyVersionHeaders(h http.Header, cfg config, etagVal, pageVer string) {
	h.Set(HeaderETag, etagVal)
	if cfg.appVersion != "" {
		h.Set(HeaderAppVersion, cfg.appVersion)
		addVaryHeader(h, cfg.clientVersionHeader)
	}
	if pageVer != "" {
		h.Set(HeaderPageVersion, pageVer)
	}
	if cfg.cacheControl != "" {
		h.Set("Cache-Control", cfg.cacheControl)
	}
}

// Middleware returns a net/http middleware that computes deterministic ETags,
// stamps X-App-Version and X-Page-Version headers, auto-busts on
// X-Client-App-Version mismatch, and serves 304 Not Modified (0-byte body) on
// If-None-Match hits.
func Middleware(opts ...Option) func(http.Handler) http.Handler {
	cfg := buildConfig(opts...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := cfg.methods[r.Method]; !ok {
				next.ServeHTTP(w, r)
				return
			}

			ew := &etagResponseWriter{
				ResponseWriter: w,
				req:            r,
				cfg:            cfg,
				statusCode:     http.StatusOK,
				outerCacheCtrl: append([]string(nil), w.Header().Values("Cache-Control")...),
			}
			next.ServeHTTP(ew, r)
			ew.finish()
		})
	}
}

type etagResponseWriter struct {
	http.ResponseWriter
	req            *http.Request
	cfg            config
	statusCode     int
	headerWritten  bool
	passthrough    bool
	buf            bytes.Buffer
	outerCacheCtrl []string
}

func (ew *etagResponseWriter) WriteHeader(code int) {
	if ew.headerWritten {
		return
	}
	ew.statusCode = code
	ew.headerWritten = true

	if isEventStream(ew.Header().Get("Content-Type")) || code < 200 || code >= 300 || code == http.StatusNoContent {
		ew.enablePassthrough()
	}
}

func (ew *etagResponseWriter) Write(p []byte) (int, error) {
	if !ew.headerWritten {
		ew.WriteHeader(http.StatusOK)
	}
	if ew.passthrough {
		return ew.ResponseWriter.Write(p)
	}
	if ew.buf.Len()+len(p) > ew.cfg.maxBufferBytes {
		ew.enablePassthrough()
		return ew.ResponseWriter.Write(p)
	}
	return ew.buf.Write(p)
}

func (ew *etagResponseWriter) Flush() {
	if !ew.passthrough {
		ew.enablePassthrough()
	}
	if f, ok := ew.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (ew *etagResponseWriter) Unwrap() http.ResponseWriter {
	return ew.ResponseWriter
}

func (ew *etagResponseWriter) enablePassthrough() {
	ew.passthrough = true
	if !ew.headerWritten {
		ew.headerWritten = true
		ew.statusCode = http.StatusOK
	}
	ew.ResponseWriter.WriteHeader(ew.statusCode)
	if ew.buf.Len() > 0 {
		_, _ = ew.ResponseWriter.Write(ew.buf.Bytes())
		ew.buf.Reset()
	}
}

func (ew *etagResponseWriter) handlerSetNoStore() bool {
	current := ew.Header().Values("Cache-Control")
	if !hasNoStore(current) {
		return false
	}
	// If an outer middleware (e.g. securityheaders.Default) set Cache-Control: no-store
	// before etag ran and the handler did not change it, allow etag's explicit
	// Cache-Control to govern ETag revalidation unless the handler set no-store itself.
	return !equalStringSlices(current, ew.outerCacheCtrl)
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (ew *etagResponseWriter) finish() {
	if ew.passthrough {
		return
	}
	if !ew.headerWritten {
		return
	}

	h := ew.Header()
	body := ew.buf.Bytes()

	if ew.handlerSetNoStore() || len(body) == 0 {
		ew.ResponseWriter.WriteHeader(ew.statusCode)
		if len(body) > 0 && ew.req.Method != http.MethodHead {
			_, _ = ew.ResponseWriter.Write(body)
		}
		return
	}

	pageVer := strings.TrimSpace(h.Get(HeaderPageVersion))
	if pageVer == "" {
		pageVer = ew.cfg.pageVersion
	}

	etagVal, shortDigest := computeTagAndDigest(body, h.Get("Content-Type"), ew.cfg, pageVer)
	if pageVer == "" {
		pageVer = shortDigest
	}

	applyVersionHeaders(h, ew.cfg, etagVal, pageVer)

	if !isClientVersionMismatch(ew.req, ew.cfg) && Match(ew.req.Header.Get(HeaderIfNoneMatch), etagVal) {
		h.Del("Content-Length")
		h.Del("Content-Type")
		ew.ResponseWriter.WriteHeader(http.StatusNotModified)
		return
	}

	h.Set("Content-Length", strconv.Itoa(len(body)))
	ew.ResponseWriter.WriteHeader(ew.statusCode)
	if ew.req.Method != http.MethodHead {
		// #nosec G705 -- Forwarding application response bytes buffered for ETag computation.
		_, _ = ew.ResponseWriter.Write(body)
	}
}
