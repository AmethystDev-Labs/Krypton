package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func NewNode(nc NodeConfig) (*Node, error) {
	u, err := url.Parse(nc.Address)
	if err != nil {
		return nil, err
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	rp.Transport = newUpstreamTransportFromConfig()
	n := &Node{
		ID:              nc.ID,
		Address:         nc.Address,
		targetURL:       u,
		Proxy:           rp,
		InitialWeight:   nc.Weight,
		effectiveWeight: nc.Weight,
		passiveScore:    100,
		activeScore:     100,
		checkScript:     nc.CheckScript,
	}
	return n, nil
}

func SetupRetryableBody(r *http.Request, maxBodySize int64) error {
	if r.Body == nil || r.Body == http.NoBody {
		return nil
	}

	bodyReader := io.LimitReader(r.Body, maxBodySize)
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if _, err := buf.ReadFrom(bodyReader); err != nil {
		return err
	}
	_ = r.Body.Close()

	bodyBytes := append([]byte(nil), buf.Bytes()...)
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}
	return nil
}

func (b *Balancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := SetupRetryableBody(r, b.config.Gateway.MaxBodySize); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	reqID := ensureRequestID(r)
	start := time.Now()
	var rw http.ResponseWriter = w
	var recorder *responseRecorder
	if LogLevel(atomic.LoadInt32(&logLevel)) == LevelDebug || b.config.Gateway.TriggerScript != "" {
		limit := 4096
		if b.config.Gateway.TriggerBodyLimit > 0 {
			limit = b.config.Gateway.TriggerBodyLimit
		}
		recorder = newResponseRecorder(w, limit)
		rw = recorder
	}

	maxRetries := b.config.Gateway.MaxRetries
	retryCfg := b.config.Gateway.Retry
	if retryCfg.MaxRetries > 0 {
		maxRetries = retryCfg.MaxRetries
	}
	if !retryCfg.Enabled {
		maxRetries = 0
	}
	if !retryCfg.EnablePost && !b.config.Gateway.RetryNonIdempotent && !isRetryableMethod(r.Method) {
		maxRetries = 0
	}

	key := r.RemoteAddr
	var lastErr error
	lastNodeID := ""
	lastRetryReason := ""
	for i := 0; i <= maxRetries; i++ {
		node := b.Select(key)
		if node == nil {
			Warnf("upstream none request_id=%s method=%s path=%s", reqID, r.Method, r.URL.Path)
			http.Error(w, "no upstream available", http.StatusServiceUnavailable)
			return
		}
		lastNodeID = node.ID

		req := r.Clone(r.Context())
		if r.GetBody != nil {
			rc, err := r.GetBody()
			if err != nil {
				lastErr = err
				break
			}
			req.Body = rc
		} else if i > 0 {
			lastErr = errors.New("retry body unavailable")
			break
		}

		var failed int32
		var stopRetry int32
		var respStatus int32
		attempt := i + 1
		total := maxRetries + 1
		canRetry := attempt < total
		proxy := *node.Proxy
		proxy.Director = func(req *http.Request) {
			target := node.targetURL
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
			req.Host = target.Host
			req.Header = r.Header.Clone()
			req.Header.Set("X-Request-Id", reqID)
			if req.Header.Get("User-Agent") == "" {
				req.Header.Set("User-Agent", "krypton")
			}
		}
		proxy.ErrorHandler = func(_ http.ResponseWriter, _ *http.Request, err error) {
			atomic.StoreInt32(&failed, 1)
			lastErr = err
			if canRetry && shouldRetryError(err, retryCfg) {
				lastRetryReason = retryReason(err)
				Warnf("upstream error request_id=%s node=%s method=%s path=%s err=%v", reqID, node.ID, r.Method, r.URL.Path, err)
				Infof("retry request_id=%s node=%s attempt=%d/%d reason=%s", reqID, node.ID, attempt, total, lastRetryReason)
			} else {
				lastRetryReason = retryReason(err)
				atomic.StoreInt32(&stopRetry, 1)
			}
			b.handleError(node, err)
		}
		proxy.ModifyResponse = func(resp *http.Response) error {
			atomic.StoreInt32(&respStatus, int32(resp.StatusCode))
			var bodyBytes []byte
			if b.config.Gateway.TriggerScript != "" || recorder != nil {
				limit := 4096
				if b.config.Gateway.TriggerBodyLimit > 0 {
					limit = b.config.Gateway.TriggerBodyLimit
				}
				if !isStreamingResponse(resp) && (resp.ContentLength >= 0 && resp.ContentLength <= int64(limit)) {
					bodyBytes, _ = io.ReadAll(resp.Body)
					_ = resp.Body.Close()
					resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					if recorder != nil {
						recorder.body = append(recorder.body[:0], bodyBytes...)
					}
				}
			}

			if b.config.Gateway.TriggerScript != "" && len(bodyBytes) > 0 {
				triggerRetry := b.runTriggerOnResponse(r, resp.StatusCode, bodyBytes, node)
				if triggerRetry && retryCfg.Enabled && canRetry {
					lastRetryReason = "trigger"
					Infof("retry request_id=%s node=%s attempt=%d/%d reason=%s", reqID, node.ID, attempt, total, lastRetryReason)
					return upstreamStatusError{StatusCode: resp.StatusCode}
				}
			}

			if retryCfg.Enabled && retryCfg.RetryOn5xx && canRetry && resp.StatusCode >= 500 && resp.StatusCode < 600 {
				lastRetryReason = "5xx"
				Infof("retry request_id=%s node=%s attempt=%d/%d reason=%s", reqID, node.ID, attempt, total, lastRetryReason)
				return upstreamStatusError{StatusCode: resp.StatusCode}
			}
			return nil
		}
		proxy.Transport = &retryTransport{base: baseTransport(node.Proxy), retry: retryCfg}

		proxy.ServeHTTP(rw, req)
		if atomic.LoadInt32(&failed) == 0 {
			status := int(atomic.LoadInt32(&respStatus))
			if status >= 500 && status < 600 {
				if status == http.StatusInternalServerError || status == http.StatusNotImplemented {
					node.UpdatePassiveScore(-5)
					node.SyncWeight(node.PassiveScore(), node.ActiveScore())
				}
			} else {
				node.UpdatePassiveScore(5)
				node.SyncWeight(node.PassiveScore(), node.ActiveScore())
			}
			logRequest(r, status, time.Since(start), recorder, reqID, node.ID)
			return
		}
		if atomic.LoadInt32(&stopRetry) == 1 {
			break
		}
	}

	if lastErr != nil {
		Warnf("upstream error request_id=%s node=%s method=%s path=%s err=%v retry_reason=%s", reqID, lastNodeID, r.Method, r.URL.Path, lastErr, lastRetryReason)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	Warnf("upstream error request_id=%s node=%s method=%s path=%s err=%v retry_reason=%s", reqID, lastNodeID, r.Method, r.URL.Path, lastErr, lastRetryReason)
	http.Error(w, "upstream error", http.StatusBadGateway)
}

func shouldRetryError(err error, cfg RetryConfig) bool {
	if !cfg.Enabled {
		return false
	}
	if isTimeout(err) {
		return cfg.RetryOnTimeout
	}
	if isConnError(err) {
		return cfg.RetryOnError
	}
	var se upstreamStatusError
	if errors.As(err, &se) {
		return cfg.RetryOn5xx
	}
	return cfg.RetryOnError
}

func retryReason(err error) string {
	if isTimeout(err) {
		return "timeout"
	}
	if isConnError(err) {
		return "conn_error"
	}
	var se upstreamStatusError
	if errors.As(err, &se) {
		return "5xx"
	}
	return "error"
}

func (b *Balancer) runTriggerOnResponse(r *http.Request, status int, body []byte, node *Node) bool {
	if b.config.Gateway.TriggerScript == "" {
		return false
	}
	req := &triggerRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: headerToMap(r.Header),
	}
	resp := &triggerResponse{
		Status: status,
		Body:   string(body),
	}
	result, err := runTrigger(r.Context(), b.config, node, req, resp)
	if err != nil {
		Warnf("trigger error request_id=%s node=%s err=%v", r.Header.Get("X-Request-Id"), node.ID, err)
		return false
	}
	if result == nil {
		return false
	}
	if result.Score != nil {
		node.SetPassiveScore(*result.Score)
	}
	if result.Penalty != nil {
		node.UpdatePassiveScore(-*result.Penalty)
	}
	if result.Reward != nil {
		node.UpdatePassiveScore(*result.Reward)
	}
	node.SyncWeight(node.PassiveScore(), node.ActiveScore())
	if result.Message != "" {
		Infof("trigger applied request_id=%s node=%s msg=%s", r.Header.Get("X-Request-Id"), node.ID, result.Message)
	}
	if result.Retry != nil && *result.Retry {
		return true
	}
	return false
}

func headerToMap(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}

func (b *Balancer) handleError(node *Node, err error) {
	switch {
	case isUpstreamRetryable(err):
		node.UpdatePassiveScore(-20)
	case isTimeout(err):
		node.UpdatePassiveScore(-30)
	case isConnError(err):
		node.UpdatePassiveScore(-50)
	default:
		node.UpdatePassiveScore(-15)
	}
	node.SyncWeight(node.PassiveScore(), node.ActiveScore())
}

func isTimeout(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

func isConnError(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && !ne.Timeout() {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}

func isRetryableMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func joinURLPath(a, b *url.URL) (path, rawpath string) {
	if a.RawPath == "" && b.RawPath == "" {
		return singleJoiningSlash(a.Path, b.Path), ""
	}
	apath := a.EscapedPath()
	bpath := b.EscapedPath()
	aslash := strings.HasSuffix(apath, "/")
	bslash := strings.HasPrefix(bpath, "/")
	switch {
	case aslash && bslash:
		return apath + bpath[1:], ""
	case !aslash && !bslash:
		return apath + "/" + bpath, ""
	}
	return apath + bpath, ""
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func isStreamingResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return true
	}
	if strings.Contains(ct, "application/x-ndjson") {
		return true
	}
	return false
}

type responseRecorder struct {
	http.ResponseWriter
	status      int
	body        []byte
	bodyLimit   int
	wroteHeader bool
}

func newResponseRecorder(w http.ResponseWriter, limit int) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
		bodyLimit:      limit,
	}
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.bodyLimit > 0 && len(r.body) < r.bodyLimit {
		remain := r.bodyLimit - len(r.body)
		if len(b) <= remain {
			r.body = append(r.body, b...)
		} else {
			r.body = append(r.body, b[:remain]...)
		}
	}
	return r.ResponseWriter.Write(b)
}

func logRequest(r *http.Request, status int, dur time.Duration, rec *responseRecorder, reqID, nodeID string) {
	Infof("request request_id=%s node=%s method=%s path=%s status=%d latency_ms=%d", reqID, nodeID, r.Method, r.URL.Path, status, dur.Milliseconds())
	if rec != nil && len(rec.body) > 0 {
		Debugf("response request_id=%s body=%s", reqID, string(rec.body))
	}
}

func readLimitedProxy(r io.Reader, limit int64) ([]byte, error) {
	limited := io.LimitReader(r, limit)
	return io.ReadAll(limited)
}

var reqCounter uint64

func ensureRequestID(r *http.Request) string {
	if rid := r.Header.Get("X-Request-Id"); rid != "" {
		return rid
	}
	id := atomic.AddUint64(&reqCounter, 1)
	rid := fmt.Sprintf("krypton-%d-%d", time.Now().UnixNano(), id)
	r.Header.Set("X-Request-Id", rid)
	return rid
}

type upstreamStatusError struct {
	StatusCode int
}

func (e upstreamStatusError) Error() string {
	return "upstream status error"
}

func isUpstreamRetryable(err error) bool {
	var se upstreamStatusError
	if errors.As(err, &se) {
		return true
	}
	return false
}

type retryTransport struct {
	base  http.RoundTripper
	retry RetryConfig
}

func (t *retryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if t.retry.Enabled && t.retry.RetryOn5xx && resp.StatusCode >= 500 && resp.StatusCode < 600 {
		_ = resp.Body.Close()
		return nil, upstreamStatusError{StatusCode: resp.StatusCode}
	}
	return resp, nil
}

func baseTransport(proxy *httputil.ReverseProxy) http.RoundTripper {
	if proxy.Transport != nil {
		return proxy.Transport
	}
	return http.DefaultTransport
}

var transportCfg transportConfig

type transportConfig struct {
	responseHeaderTimeout time.Duration
	idleConnTimeout       time.Duration
	upstreamTimeout       time.Duration
	maxIdleConns          int
	maxIdleConnsPerHost   int
	maxConnsPerHost       int
}

func setTransportConfig(cfg *Config) {
	transportCfg = transportConfig{
		responseHeaderTimeout: cfg.Gateway.ResponseHeaderTimeout.Duration,
		idleConnTimeout:       cfg.Gateway.IdleConnTimeout.Duration,
		upstreamTimeout:       cfg.Gateway.UpstreamTimeout.Duration,
		maxIdleConns:          cfg.Gateway.MaxIdleConns,
		maxIdleConnsPerHost:   cfg.Gateway.MaxIdleConnsPerHost,
		maxConnsPerHost:       cfg.Gateway.MaxConnsPerHost,
	}
}

func newUpstreamTransportFromConfig() http.RoundTripper {
	rc := transportCfg
	dialTimeout := rc.upstreamTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	rhTimeout := rc.responseHeaderTimeout
	if rhTimeout <= 0 {
		rhTimeout = 10 * time.Second
	}
	idleTimeout := rc.idleConnTimeout
	if idleTimeout <= 0 {
		idleTimeout = 90 * time.Second
	}
	maxIdle := rc.maxIdleConns
	if maxIdle <= 0 {
		maxIdle = 256
	}
	maxIdleHost := rc.maxIdleConnsPerHost
	if maxIdleHost <= 0 {
		maxIdleHost = 64
	}

	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxIdle,
		MaxIdleConnsPerHost:   maxIdleHost,
		MaxConnsPerHost:       rc.maxConnsPerHost,
		IdleConnTimeout:       idleTimeout,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: rhTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
