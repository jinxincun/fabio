package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func newHTTPProxy(t *url.URL, tr http.RoundTripper, flush time.Duration) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(t)
	rp.Transport = tr
	rp.FlushInterval = flush
	rp.Transport = &transport{tr, nil}
	return &httpHandler{rp}
}

// responser exposes the response from an HTTP request.
type responser interface {
	response() *http.Response
}

// httpHandler is a simple wrapper around a reverse proxy to access the
// captured response object in the underlying transport object. There
// may be a better way of doing this.
type httpHandler struct {
	rp *httputil.ReverseProxy
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.rp.ServeHTTP(w, r)
}

func (h *httpHandler) response() *http.Response {
	return h.rp.Transport.(*transport).resp
}

// transport executes the roundtrip and captures the response. It is not
// safe for multiple or concurrent use since it only captures a single
// response.
type transport struct {
	http.RoundTripper
	resp *http.Response
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(r)
	t.resp = resp
	return resp, err
}
