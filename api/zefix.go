package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/matthiasak/chli/config"
)

const (
	zefixBaseURL  = "https://www.zefix.admin.ch/ZefixPublicREST"
	zefixCacheTTL = 6 * time.Hour
)

// ZefixSearch searches the Swiss commercial register by company name.
func (c *Client) ZefixSearch(name, canton, lang string, maxEntries int) ([]ZefixCompany, error) {
	if maxEntries <= 0 {
		maxEntries = 30
	}
	reqBody := ZefixSearchRequest{
		Name:          name,
		Canton:        strings.ToUpper(canton),
		LanguageKey:   mapZefixLang(lang),
		DeletedFilter: "ACTIVE",
		MaxEntries:    maxEntries,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	url := zefixBaseURL + "/api/v1/company/search"
	cacheKey := "POST:" + url + ":" + string(bodyBytes)
	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			var out []ZefixCompany
			if err := json.Unmarshal(data, &out); err == nil {
				return out, nil
			}
		}
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(string(bodyBytes))), nil
	}
	applyZefixAuth(req)

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, fmt.Errorf("could not reach Zefix: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("%s", zefixAuthNote)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Zefix API error %d: %s", resp.StatusCode, string(b))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Zefix may wrap results in {"list":[...]} OR return a bare array.
	var out []ZefixCompany
	if err := json.Unmarshal(data, &out); err != nil {
		var wrapped struct {
			List []ZefixCompany `json:"list"`
		}
		if err2 := json.Unmarshal(data, &wrapped); err2 != nil {
			return nil, fmt.Errorf("parsing Zefix response: %w", err)
		}
		out = wrapped.List
	}

	c.Cache.Set(cacheKey, data, zefixCacheTTL)
	return out, nil
}

// ZefixCompanyByUID fetches all commercial register entries for a UID.
func (c *Client) ZefixCompanyByUID(uid string) ([]ZefixCompany, error) {
	return c.zefixGetList("/api/v1/company/uid/" + NormalizeUID(uid))
}

// ZefixCompanyByCHID fetches a company by its cantonal CH-ID.
func (c *Client) ZefixCompanyByCHID(chid string) ([]ZefixCompany, error) {
	return c.zefixGetList("/api/v1/company/chid/" + chid)
}

func (c *Client) zefixGetList(path string) ([]ZefixCompany, error) {
	url := zefixBaseURL + path
	cacheKey := "GET:" + url
	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			var out []ZefixCompany
			if err := json.Unmarshal(data, &out); err == nil {
				return out, nil
			}
		}
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	applyZefixAuth(req)

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, fmt.Errorf("could not reach Zefix: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("%s", zefixAuthNote)
	}
	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Zefix API error %d: %s", resp.StatusCode, string(b))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	var out []ZefixCompany
	if err := json.Unmarshal(data, &out); err != nil {
		// Single-object endpoints sometimes return a map; wrap it.
		var single ZefixCompany
		if err2 := json.Unmarshal(data, &single); err2 == nil && single.UID != "" {
			out = []ZefixCompany{single}
		} else {
			return nil, fmt.Errorf("parsing Zefix response: %w", err)
		}
	}
	c.Cache.Set(cacheKey, data, zefixCacheTTL)
	return out, nil
}

// NormalizeUID canonicalises user input ("CHE123456789", "che-123.456.789",
// "123456789") to the Zefix-expected form "CHE123456789".
func NormalizeUID(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, " ", "")
	if !strings.HasPrefix(s, "CHE") && len(s) == 9 {
		s = "CHE" + s
	}
	return s
}

// FormatUID renders a canonical UID as "CHE-123.456.789".
func FormatUID(uid string) string {
	s := NormalizeUID(uid)
	if len(s) != 12 || !strings.HasPrefix(s, "CHE") {
		return uid
	}
	return fmt.Sprintf("CHE-%s.%s.%s", s[3:6], s[6:9], s[9:12])
}

// applyZefixAuth attaches HTTP Basic auth from, in order:
//  1. ZEFIX_USER / ZEFIX_PASS env vars
//  2. credentials stored via `chli zefix login`
//
// Without credentials the Zefix REST API returns 401.
func applyZefixAuth(req *http.Request) {
	user := os.Getenv("ZEFIX_USER")
	pass := os.Getenv("ZEFIX_PASS")
	if user == "" || pass == "" {
		if creds, err := config.LoadCredentials(); err == nil {
			c := creds.Get("zefix")
			if user == "" {
				user = c.User
			}
			if pass == "" {
				pass = c.Password
			}
		}
	}
	if user == "" || pass == "" {
		return
	}
	req.SetBasicAuth(user, pass)
}

// zefixAuthNote is returned to the user when a 401 is seen.
const zefixAuthNote = "Zefix requires HTTP Basic auth. Register for API access via https://www.zefix.admin.ch/ (see the API section) to obtain credentials, then run `chli zefix login` (or `chli uid login`) or export ZEFIX_USER and ZEFIX_PASS."

func mapZefixLang(lang string) string {
	switch lang {
	case "fr":
		return "fr"
	case "it":
		return "it"
	case "en":
		return "en"
	default:
		return "de"
	}
}
