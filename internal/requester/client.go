package requester

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

type Config struct {
	Concurrency      int
	Timeout          time.Duration
	DisableKeepAlive bool
}

type Result struct {
	StatusCode int
	Duration   time.Duration
	BytesRead  int64
	Error      error
}

type Requester struct {
	client *http.Client
}

func NewRequester(cfg Config) *Requester {
	transport := &http.Transport{
		MaxIdleConns:        cfg.Concurrency,
		MaxIdleConnsPerHost: cfg.Concurrency,
		IdleConnTimeout:     30 - time.Second,
		DisableKeepAlives:   cfg.DisableKeepAlive,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	return &Requester{
		client: client,
	}
}

func (r *Requester) Do(method, url string, body []byte, headers map[string]string) Result {
	start := time.Now()

	var br io.Reader
	if len(body) > 0 {
		br = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, br)
	if err != nil {
		return Result{Error: err, Duration: time.Since(start)}
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := r.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return Result{Error: err, Duration: duration}
	}

	defer resp.Body.Close()

	written, _ := io.Copy(io.Discard, resp.Body)

	return Result{
		Duration:   duration,
		BytesRead:  written,
		StatusCode: resp.StatusCode,
	}
}
