// Package jira provides an HTTP client for the Jira Cloud REST API.
package jira

import (
	"bytes"
	"io"
	"net/http"
	"time"

	jira "github.com/andygrunwald/go-jira/v2/cloud"
	"github.com/rs/zerolog"
)

// loggingTransport wraps an http.RoundTripper and logs requests and responses at trace level.
type loggingTransport struct {
	inner  http.RoundTripper
	logger zerolog.Logger
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	resp, err := t.inner.RoundTrip(req)
	elapsed := time.Since(start)

	ev := t.logger.Trace().
		Str("method", req.Method).
		Str("url", req.URL.String()).
		Str("elapsed", elapsed.String())
	if len(reqBody) > 0 {
		ev.RawJSON("req_body", reqBody)
	}
	if err != nil {
		ev.Err(err).Msg("http round trip failed")
		return nil, err
	}
	ev.Int("status", resp.StatusCode)
	if resp.Body != nil {
		respBody, _ := io.ReadAll(resp.Body)
		err = resp.Body.Close()
		if err != nil {
			ev.Err(err).Msg("failed to close response body")
		}
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		if len(respBody) > 1000 {
			respBody = respBody[:1000]
			respBody = append(respBody, []byte("...[truncated]")...)
		}
		ev.Str("resp_body", string(respBody))
	}
	ev.Msg("http round trip")
	return resp, nil
}

// Client communicates with the Jira Cloud REST API (go-jira v2 cloud client).
type Client struct {
	*jira.Client
	logger zerolog.Logger
	token  string
	email  string
}

// NewClient creates a new Jira API client.
func NewClient(
	baseURL, email, token string,
	logger zerolog.Logger,
) (*Client, error) {
	l := logger.With().Str("component", "jira").Logger()

	transport := jira.BasicAuthTransport{
		Username: email,
		APIToken: token,
		Transport: &loggingTransport{
			inner:  http.DefaultTransport,
			logger: l,
		},
	}

	client, err := jira.NewClient(baseURL, transport.Client())
	if err != nil {
		return nil, err
	}
	return &Client{Client: client, logger: l, email: email, token: token}, nil
}
