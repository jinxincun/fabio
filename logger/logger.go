// Package logger implements a configurable access logger.
//
// The access log format is defined through a format string which
// expands to a log line per request.
//
//   header.<name>           - request http header (name: [a-zA-Z0-9-]+)
//   remote_addr             - host:port of remote client
//   remote_host             - host of remote client
//   remote_port             - port of remote client
//   request                 - request <method> <uri> <proto>
//   request_args            - request query parameters
//   request_host            - request host header (aka server name)
//   request_method          - request method
//   request_scheme          - request scheme
//   request_uri             - request URI
//   request_url             - request URL
//   request_proto           - request protocol
//   response_body_size      - response body size in bytes
//   response_status         - response status code
//   response_time_ms        - response time in S.sss format
//   response_time_us        - response time in S.ssssss format
//   response_time_ns        - response time in S.sssssssss format
//   time_rfc3339            - log timestamp in YYYY-MM-DDTHH:MM:SSZ format
//   time_rfc3339_ms         - log timestamp in YYYY-MM-DDTHH:MM:SS.sssZ format
//   time_rfc3339_us         - log timestamp in YYYY-MM-DDTHH:MM:SS.ssssssZ format
//   time_rfc3339_ns         - log timestamp in YYYY-MM-DDTHH:MM:SS.sssssssssZ format
//   time_unix_ms            - log timestamp in unix epoch ms
//   time_unix_us            - log timestamp in unix epoch us
//   time_unix_ns            - log timestamp in unix epoch ns
//   time_common             - log timestamp in DD/MMM/YYYY:HH:MM:SS -ZZZZ
//   upstream_addr           - host:port of upstream server
//   upstream_host           - host of upstream server
//   upstream_port           - port of upstream server
//   upstream_request_scheme - upstream request scheme
//   upstream_request_uri    - upstream request URI
//   upstream_request_url    - upstream request URL
//
package logger

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

const (
	// Common defines the Apache Common Log file format.
	Common = `$remote_host - - [$time_common] "$request" $response_status $response_body_size`

	// Combined defines the Apache Combined Log file format.
	Combined = `$remote_host - - [$time_common] "$request" $response_status $response_body_size "$header.Referer" "$header.User-Agent"`
)

// Event defines the elements of a loggable event.
type Event struct {
	// Start is the time when the action that triggered the event started.
	Start time.Time

	// End is the time when the action that triggered the event was completed.
	End time.Time

	// Request is the HTTP request that is connected to this event.
	Request *http.Request

	// Response is the HTTP response which is connected to this event.
	Response *http.Response

	// RequestURL is the URL of the incoming HTTP request.
	RequestURL *url.URL

	// UpstreamAddr is the TCP address in the form of "host:port" of the
	// upstream server which handled the proxied request.
	UpstreamAddr string

	// UpstreamURL is the URL which was sent to the upstream server.
	UpstreamURL *url.URL
}

// Logger logs an event.
type Logger interface {
	Log(*Event)
}

// New creates a new logger that writes log events in the
// given format to the provided writer. If the format
// is empty or invalid an error is returned.
func New(w io.Writer, format string) (Logger, error) {
	p, err := parse(format, Fields)
	if err != nil {
		return nil, err
	}
	if len(p) == 0 {
		return nil, errors.New("empty log format")
	}
	if w == nil {
		w = os.Stdout
	}
	return &logger{p: p, w: w}, nil
}

type logger struct {
	p pattern

	mu sync.Mutex
	w  io.Writer
}

// BufSize defines the default size of the log buffers.
const BufSize = 1024

// pool provides a reusable set of log buffers.
var pool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, BufSize))
	},
}

// Log writes a log line for the request that was executed
// between t1 and t2.
func (l *logger) Log(e *Event) {
	b := pool.Get().(*bytes.Buffer)
	b.Reset()
	l.p.write(b, e)
	l.mu.Lock()
	l.w.Write(b.Bytes())
	l.mu.Unlock()
	pool.Put(b)
}

// atoi is a replacement for strconv.Atoi/strconv.FormatInt
// which does not alloc.
func atoi(b *bytes.Buffer, i int64, pad int) {
	var flag bool
	if i < 0 {
		flag = true
		i = -i
	}

	// format number
	// 2^63-1 == 9223372036854775807
	var d [128]byte
	n, p := len(d), len(d)-1
	for i >= 0 {
		d[p] = byte('0') + byte(i%10)
		i /= 10
		p--
		if i == 0 {
			break
		}
	}

	// padding
	for n-p-1 < pad {
		d[p] = byte('0')
		p--
	}

	if flag {
		d[p] = '-'
		p--
	}
	b.Write(d[p+1:])
}
