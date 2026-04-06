package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/matthiasak/chli/config"
	"github.com/matthiasak/chli/output"
)

var version = "dev"

type Client struct {
	HTTP    *http.Client
	Config  *config.Config
	Cache   *Cache
	NoCache bool
	Refresh bool
}

func NewClient() (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	cache, err := NewCache(cfg.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("initializing cache: %w", err)
	}

	timeout := 30 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	// Custom TLS config that mimics a browser TLS fingerprint.
	// This avoids WAF blocks on Swiss government sites (e.g. parlament.ch)
	// that reject Go's default HTTP/2 TLS fingerprint.
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}

	transport := &http.Transport{
		ForceAttemptHTTP2: false, // Force HTTP/1.1 for WAF compatibility
		TLSClientConfig:  tlsConfig,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:  10 * time.Second,
		ResponseHeaderTimeout: timeout,
	}

	return &Client{
		HTTP: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		Config: cfg,
		Cache:  cache,
	}, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "de-CH,de;q=0.9,en;q=0.8")
	}
	output.Debugf("HTTP %s %s", req.Method, req.URL.String())
	return c.HTTP.Do(req)
}

// doWithRetry executes a request with exponential backoff retry on transient errors.
func (c *Client) doWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 500 * time.Millisecond
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			delay := backoff + jitter
			output.Verbosef("Retry %d/%d after %s...", attempt, maxRetries, delay)
			time.Sleep(delay)

			// Clone the request for retry (body may have been consumed)
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("preparing retry: %w", err)
				}
				req.Body = body
			}
		}

		resp, err := c.do(req)
		if err != nil {
			lastErr = err
			output.Debugf("Request failed (attempt %d): %v", attempt+1, err)
			continue
		}

		// Retry on 429 (rate limit) or 5xx (server errors)
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
			output.Debugf("Server error %d (attempt %d)", resp.StatusCode, attempt+1)
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("could not reach API after %d attempts: %w", maxRetries+1, lastErr)
}

func (c *Client) DoJSON(baseURL, path string, result any) error {
	return c.DoJSONWithTTL(baseURL, path, result, 24*time.Hour)
}

func (c *Client) DoJSONWithTTL(baseURL, path string, result any, ttl time.Duration) error {
	url := baseURL + path

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get("GET:" + url); ok {
			output.Debugf("Cache hit: %s", url)
			return json.Unmarshal(data, result)
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set("GET:"+url, data, ttl)

	return json.Unmarshal(data, result)
}

func (c *Client) DoRaw(baseURL, path string) ([]byte, error) {
	return c.DoRawWithTTL(baseURL, path, 24*time.Hour)
}

func (c *Client) DoRawWithTTL(baseURL, path string, ttl time.Duration) ([]byte, error) {
	url := baseURL + path

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get("GET:" + url); ok {
			output.Debugf("Cache hit: %s", url)
			return data, nil
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set("GET:"+url, data, ttl)

	return data, nil
}

func (c *Client) DoPost(baseURL, path string, contentType string, body string, result any, ttl time.Duration) error {
	url := baseURL + path
	cacheKey := "POST:" + url + ":" + body

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			output.Debugf("Cache hit: POST %s", url)
			return json.Unmarshal(data, result)
		}
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(body)), nil
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set(cacheKey, data, ttl)

	return json.Unmarshal(data, result)
}

func (c *Client) DoPostRaw(baseURL, path string, contentType string, body string, ttl time.Duration) ([]byte, error) {
	url := baseURL + path
	cacheKey := "POST:" + url + ":" + body

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			output.Debugf("Cache hit: POST %s", url)
			return data, nil
		}
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(body)), nil
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set(cacheKey, data, ttl)

	return data, nil
}

// DoGetWithHeaders performs a GET with custom headers.
func (c *Client) DoGetWithHeaders(url string, headers map[string]string, result any, ttl time.Duration) error {
	cacheKey := "GET:" + url

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			output.Debugf("Cache hit: %s", url)
			return json.Unmarshal(data, result)
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set(cacheKey, data, ttl)

	return json.Unmarshal(data, result)
}

// DoGetRawWithHeaders performs a GET with custom headers and returns raw bytes.
func (c *Client) DoGetRawWithHeaders(url string, headers map[string]string, ttl time.Duration) ([]byte, error) {
	cacheKey := "GET:" + url

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			output.Debugf("Cache hit: %s", url)
			return data, nil
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set(cacheKey, data, ttl)

	return data, nil
}

// DownloadFile downloads a URL to a local file path.
func (c *Client) DownloadFile(url string, headers map[string]string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download error %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	return data, contentType, nil
}
