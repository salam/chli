package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/matthiasak/chli/config"
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
	// Force HTTP/1.1 — some Swiss government WAFs (e.g. parlament.ch) block
	// Go's default HTTP/2 TLS fingerprint.
	transport := &http.Transport{
		ForceAttemptHTTP2: false,
	}
	return &Client{
		HTTP: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		Config: cfg,
		Cache:  cache,
	}, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; chli/"+version+"; +https://github.com/matthiasak/chli)")
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	return c.HTTP.Do(req)
}

func (c *Client) DoJSON(baseURL, path string, result any) error {
	return c.DoJSONWithTTL(baseURL, path, result, 24*time.Hour)
}

func (c *Client) DoJSONWithTTL(baseURL, path string, result any, ttl time.Duration) error {
	url := baseURL + path

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get("GET:" + url); ok {
			return json.Unmarshal(data, result)
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("could not reach API: check your internet connection")
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
			return data, nil
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach API: check your internet connection")
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
			return json.Unmarshal(data, result)
		}
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("could not reach API: check your internet connection")
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
			return data, nil
		}
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach API: check your internet connection")
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

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("could not reach API: check your internet connection")
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

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach API: check your internet connection")
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
