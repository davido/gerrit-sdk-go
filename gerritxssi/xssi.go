// Package gerritxssi strips Gerrit's XSSI guard from HTTP responses.
//
// Every Gerrit JSON body starts with the magic prefix )]}' on its own line, to
// defeat cross-site script inclusion. That prefix is not valid JSON and is not
// expressible in OpenAPI, so the generated client cannot know about it. This is the
// one Gerrit-specific step: a transport wrapper that removes the prefix before the
// generated decoder sees the body -- no edits to generated code, so it survives
// regeneration.
package gerritxssi

import (
	"bytes"
	"io"
	"net/http"
)

var guard = []byte(")]}'\n")

type transport struct{ base http.RoundTripper }

// NewTransport wraps base (or http.DefaultTransport when base is nil) so that the
// Gerrit XSSI guard is stripped from every response body.
func NewTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &transport{base}
}

// Client returns an *http.Client whose transport strips the guard. Assign it to
// gerritclient.Configuration.HTTPClient.
func Client() *http.Client {
	return &http.Client{Transport: NewTransport(nil)}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.Body == nil {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	// TrimPrefix is a no-op on text/plain and binary bodies (they don't start with
	// the guard), so this is safe to apply to every response.
	body = bytes.TrimPrefix(body, guard)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}
