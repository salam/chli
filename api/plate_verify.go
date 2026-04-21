package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// plateVerifyUA is the User-Agent used for endpoint verification.
// The %s is replaced with the binary version at call time.
const plateVerifyUATemplate = "chli-plate-verify/%s (+https://github.com/salam/chli)"

// plateVerifyBodyKeywords are the fuzzy body-text markers we expect on a
// canton Halterauskunft landing page. Case-insensitive contains match.
var plateVerifyBodyKeywords = []string{
	"halterauskunft",
	"halter",
	"détenteur",
	"detentore",
}

// plateVerifyBodyLimit caps body reads (1 MiB).
const plateVerifyBodyLimit = 1 << 20

// VerifyCantonOpts lets callers override the HTTP client (used by tests).
type VerifyCantonOpts struct {
	HTTPClient *http.Client
	Version    string
}

// VerifyCanton performs an observational probe of a canton's halterauskunft
// landing page (and form_pdf if set). It never submits the form, never solves
// captcha. Returns a VerifyResult with status ok|warn|error and reasons.
func VerifyCanton(ctx context.Context, c CantonEntry) VerifyResult {
	return VerifyCantonWith(ctx, c, VerifyCantonOpts{})
}

// VerifyCantonWith is VerifyCanton with overrideable HTTP client for testing.
func VerifyCantonWith(ctx context.Context, c CantonEntry, opts VerifyCantonOpts) VerifyResult {
	started := time.Now()
	res := VerifyResult{
		Canton:    c.Code,
		URL:       c.Halterauskunft.URL,
		Status:    VerifyOK,
		CheckedAt: started.UTC().Format(time.RFC3339),
	}

	// Cantons with no URL are not verifiable — emit a warn so CI doesn't
	// flag them as errors but still notices they're stubs.
	if c.Halterauskunft.Mode == ModeUnavailable || strings.TrimSpace(c.Halterauskunft.URL) == "" {
		res.URL = c.Halterauskunft.URL
		res.Status = VerifyWarn
		res.Reasons = append(res.Reasons, fmt.Sprintf("no halterauskunft URL for canton %s (mode=%s)", c.Code, c.Halterauskunft.Mode))
		res.ElapsedMS = time.Since(started).Milliseconds()
		return res
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	ver := opts.Version
	if ver == "" {
		ver = version
		if ver == "" {
			ver = "dev"
		}
	}
	ua := fmt.Sprintf(plateVerifyUATemplate, ver)

	probe := func(url string) (int, string, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return 0, "", nil, err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/pdf,*/*;q=0.8")
		req.Header.Set("Accept-Language", "de-CH,de;q=0.9,fr;q=0.8,it;q=0.7,en;q=0.6")
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, plateVerifyBodyLimit))
		finalURL := url
		if resp.Request != nil && resp.Request.URL != nil {
			finalURL = resp.Request.URL.String()
		}
		return resp.StatusCode, finalURL, body, nil
	}

	status, finalURL, body, err := probe(c.Halterauskunft.URL)
	res.HTTPStatus = status
	res.FinalURL = finalURL
	if err != nil {
		res.Status = VerifyError
		var reason string
		if errors.Is(err, context.DeadlineExceeded) {
			reason = "request timed out"
		} else {
			reason = fmt.Sprintf("request failed: %v", err)
		}
		res.Reasons = append(res.Reasons, reason)
		res.ElapsedMS = time.Since(started).Milliseconds()
		return res
	}

	if status >= 500 {
		res.Status = VerifyError
		res.Reasons = append(res.Reasons, fmt.Sprintf("HTTP %d from canton endpoint", status))
	} else if status >= 400 {
		res.Status = VerifyError
		res.Reasons = append(res.Reasons, fmt.Sprintf("HTTP %d from canton endpoint", status))
	} else if status >= 300 {
		// 3xx without a redirect followed is unusual; the http.Client
		// follows by default, so we'd normally see a final 2xx. Warn.
		res.Status = worseStatus(res.Status, VerifyWarn)
		res.Reasons = append(res.Reasons, fmt.Sprintf("HTTP %d (redirect not followed)", status))
	}

	// Hostname-changed detection: the spec calls out hostname changes as an
	// error because deep links probably no longer resolve against canton data.
	if u := strings.TrimSpace(finalURL); u != "" && u != c.Halterauskunft.URL {
		if hostOf(finalURL) != hostOf(c.Halterauskunft.URL) {
			res.Status = worseStatus(res.Status, VerifyWarn)
			res.Reasons = append(res.Reasons, fmt.Sprintf("final URL host %q differs from configured %q", hostOf(finalURL), hostOf(c.Halterauskunft.URL)))
		}
	}

	// Body heuristics — only on 2xx responses.
	if status >= 200 && status < 300 {
		text := strings.ToLower(string(body))
		cantonName := strings.ToLower(c.Name("de"))
		if cantonName != "" && !strings.Contains(text, cantonName) {
			res.Status = worseStatus(res.Status, VerifyWarn)
			res.Reasons = append(res.Reasons, fmt.Sprintf("canton name %q not found in body", c.Name("de")))
		}
		if !containsAny(text, plateVerifyBodyKeywords) {
			res.Status = worseStatus(res.Status, VerifyWarn)
			res.Reasons = append(res.Reasons, "no halterauskunft keyword found in body")
		}
	}

	// form_pdf (if present) — lighter check, only that it responds.
	if c.Halterauskunft.FormPDF != "" {
		pdfStatus, _, _, pdfErr := probe(c.Halterauskunft.FormPDF)
		if pdfErr != nil {
			res.Status = worseStatus(res.Status, VerifyError)
			res.Reasons = append(res.Reasons, fmt.Sprintf("form_pdf fetch failed: %v", pdfErr))
		} else if pdfStatus >= 400 {
			res.Status = worseStatus(res.Status, VerifyError)
			res.Reasons = append(res.Reasons, fmt.Sprintf("form_pdf HTTP %d", pdfStatus))
		}
	}

	res.ElapsedMS = time.Since(started).Milliseconds()
	return res
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if n != "" && strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// worseStatus returns the more severe of a and b. error > warn > ok.
func worseStatus(a, b VerifyStatus) VerifyStatus {
	rank := func(s VerifyStatus) int {
		switch s {
		case VerifyError:
			return 2
		case VerifyWarn:
			return 1
		}
		return 0
	}
	if rank(a) >= rank(b) {
		return a
	}
	return b
}

// hostOf returns the host portion of a URL string without importing net/url
// for just this — cheap and tolerant. Returns "" on unparseable input.
func hostOf(u string) string {
	s := u
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}
