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
	"github.com/matthiasak/chli/output"
)

const (
	zefixBaseURL       = "https://www.zefix.admin.ch/ZefixPublicREST"
	zefixPublicBaseURL = "https://zefix.ch/ZefixREST"
	zefixCacheTTL      = 6 * time.Hour
)

// ZefixSearch searches the Swiss commercial register by company name.
//
// Routing: uses the authenticated official API when credentials are
// configured; otherwise falls back to the unauthenticated zefix.ch endpoints
// used by the public website. A 401 from the official API also triggers the
// fallback.
func (c *Client) ZefixSearch(name, canton, lang string, maxEntries int) ([]ZefixCompany, error) {
	if maxEntries <= 0 {
		maxEntries = 30
	}
	langKey := mapZefixLang(lang)
	cantonCode := strings.ToUpper(canton)

	if !hasZefixCreds() {
		return c.zefixSearchPublic(name, cantonCode, langKey, maxEntries)
	}

	reqBody := ZefixSearchRequest{
		Name:          name,
		Canton:        cantonCode,
		LanguageKey:   langKey,
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
		output.Debugf("Zefix official API returned 401, falling back to public endpoint")
		return c.zefixSearchPublic(name, cantonCode, langKey, maxEntries)
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
	uid = NormalizeUID(uid)
	if !hasZefixCreds() {
		return c.zefixSearchPublic(uid, "", "de", 30)
	}
	results, err := c.zefixGetList("/api/v1/company/uid/" + uid)
	if err != nil && strings.Contains(err.Error(), "401") {
		return c.zefixSearchPublic(uid, "", "de", 30)
	}
	return results, err
}

// ZefixCompanyByCHID fetches a company by its cantonal CH-ID.
func (c *Client) ZefixCompanyByCHID(chid string) ([]ZefixCompany, error) {
	if !hasZefixCreds() {
		return c.zefixSearchPublic(chid, "", "de", 30)
	}
	results, err := c.zefixGetList("/api/v1/company/chid/" + chid)
	if err != nil && strings.Contains(err.Error(), "401") {
		return c.zefixSearchPublic(chid, "", "de", 30)
	}
	return results, err
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
		return nil, fmt.Errorf("Zefix API error 401: unauthorized")
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

// zefixSearchPublic queries the unauthenticated endpoints used by zefix.ch.
// The public endpoint's `name` field matches on company name, UID (with or
// without punctuation), and CHID — so it doubles as our UID/CHID lookup when
// the official API is unavailable. The `canton` filter is silently ignored
// server-side; results are post-filtered here when one is requested.
func (c *Client) zefixSearchPublic(query, canton, lang string, maxEntries int) ([]ZefixCompany, error) {
	if maxEntries <= 0 {
		maxEntries = 30
	}
	payload := map[string]any{
		"languageKey": lang,
		"maxEntries":  maxEntries,
		"offset":      0,
		"name":        query,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	url := zefixPublicBaseURL + "/api/v1/firm/search.json"
	cacheKey := "POST:" + url + ":" + string(bodyBytes)
	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			return decodeZefixPublicList(data, canton)
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

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, fmt.Errorf("could not reach Zefix public endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Zefix public API error %d: %s", resp.StatusCode, string(b))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set(cacheKey, data, zefixCacheTTL)
	return decodeZefixPublicList(data, canton)
}

func decodeZefixPublicList(data []byte, cantonFilter string) ([]ZefixCompany, error) {
	var wrapped struct {
		List []zefixPublicFirm `json:"list"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("parsing Zefix public response: %w", err)
	}
	out := make([]ZefixCompany, 0, len(wrapped.List))
	for _, f := range wrapped.List {
		c := f.toZefixCompany()
		if cantonFilter != "" && !strings.EqualFold(c.Canton, cantonFilter) {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// zefixPublicFirm mirrors the list-item shape returned by the public endpoint.
// It differs from ZefixCompany mainly by flattening legal form / register
// office to bare ids and by surfacing cantonalExcerptWeb, which we use to
// derive the canton code that the public API does not return directly.
type zefixPublicFirm struct {
	Name               string `json:"name"`
	EhraID             int64  `json:"ehraid"`
	UID                string `json:"uid"`
	UIDFormatted       string `json:"uidFormatted"`
	CHID               string `json:"chid"`
	LegalSeatID        int    `json:"legalSeatId"`
	LegalSeat          string `json:"legalSeat"`
	RegisterOfficeID   int    `json:"registerOfficeId"`
	LegalFormID        int    `json:"legalFormId"`
	Status             string `json:"status"`
	ShabDate           string `json:"shabDate"`
	DeleteDate         string `json:"deleteDate"`
	CantonalExcerptWeb string `json:"cantonalExcerptWeb"`
}

func (f zefixPublicFirm) toZefixCompany() ZefixCompany {
	return ZefixCompany{
		EhraID:       f.EhraID,
		CHID:         f.CHID,
		UID:          f.UID,
		UIDFormatted: f.UIDFormatted,
		LegalSeat:    f.LegalSeat,
		LegalSeatID:  f.LegalSeatID,
		Name:         f.Name,
		Canton:       cantonFromChRegisterURL(f.CantonalExcerptWeb),
		Status:       f.Status,
		SogcDate:     f.ShabDate,
		DeletionDate: f.DeleteDate,
	}
}

// cantonFromChRegisterURL extracts the canton code from a cantonal register
// excerpt URL such as "https://zh.chregister.ch/..." → "ZH". The public Zefix
// API does not return the canton code directly on list results, but every
// active entry carries this URL. Returns "" when the URL does not match the
// expected chregister.ch pattern (which is the case for a handful of cantons
// that use different hostnames; callers should treat Canton as best-effort).
func cantonFromChRegisterURL(u string) string {
	const marker = ".chregister.ch"
	i := strings.Index(u, marker)
	if i < 0 {
		return ""
	}
	// Walk back to the scheme separator or start.
	start := strings.LastIndex(u[:i], "/")
	if start < 0 {
		return ""
	}
	host := u[start+1 : i]
	if len(host) != 2 {
		return ""
	}
	return strings.ToUpper(host)
}

// hasZefixCreds reports whether ZEFIX_USER/ZEFIX_PASS env vars or stored
// credentials are available. Used to decide whether to try the authenticated
// official API at all.
func hasZefixCreds() bool {
	if os.Getenv("ZEFIX_USER") != "" && os.Getenv("ZEFIX_PASS") != "" {
		return true
	}
	creds, err := config.LoadCredentials()
	if err != nil {
		return false
	}
	c := creds.Get("zefix")
	return c.User != "" && c.Password != ""
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
