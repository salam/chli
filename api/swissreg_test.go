package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matthiasak/chli/config"
)

// newTestClient returns a Client whose cache is writable but disabled.
func newTestClient(t *testing.T) *Client {
	t.Helper()
	cache, err := NewCache(t.TempDir())
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	return &Client{
		HTTP:    http.DefaultClient,
		Config:  &config.Config{},
		Cache:   cache,
		NoCache: true, // avoid cross-test pollution
	}
}

// Rewrite the swissreg host at runtime for a test. Returns a cleanup fn.
func pointSwissregAt(t *testing.T, srv *httptest.Server) func() {
	t.Helper()
	orig := swissregBaseHostOverride
	swissregBaseHostOverride = srv.URL
	return func() { swissregBaseHostOverride = orig }
}

func TestSwissregSearch_Request(t *testing.T) {
	var gotPath, gotMethod, gotCT, gotVers string
	var gotBody swissregReqBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		gotVers = r.Header.Get("x-ipi-version")
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &gotBody); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"target":"chmarke","totalItems":1,"pageSize":16,
			"results":[{"id":["urn:ige:schutztitel:chmarke:1"],"titel__type_text":["AMBIA"]}]
		}`))
	}))
	defer srv.Close()
	defer pointSwissregAt(t, srv)()

	c := newTestClient(t)
	resp, err := c.SwissregSearch(SwissregQuery{
		Target:       "chmarke",
		SearchString: "Ambia",
		Filters:      map[string][]string{"schutztitelstatus__type_i18n": {"aktiv"}},
		PageSize:     10,
	})
	if err != nil {
		t.Fatalf("SwissregSearch: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != swissregSearch {
		t.Errorf("path = %q, want %q", gotPath, swissregSearch)
	}
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Errorf("content-type = %q", gotCT)
	}
	if gotVers == "" {
		t.Errorf("x-ipi-version header missing")
	}
	if gotBody.Target != "chmarke" || gotBody.SearchString != "Ambia" {
		t.Errorf("body target/search wrong: %+v", gotBody)
	}
	if gotBody.PageSize != 10 {
		t.Errorf("pageSize = %d, want 10", gotBody.PageSize)
	}
	if got := gotBody.Filters["schutztitelstatus__type_i18n"]; len(got) != 1 || got[0] != "aktiv" {
		t.Errorf("filter not forwarded: %+v", gotBody.Filters)
	}
	if resp.TotalItems != 1 || len(resp.Results) != 1 {
		t.Errorf("unexpected response: %+v", resp)
	}
	if got := resp.Results[0].First("titel__type_text"); got != "AMBIA" {
		t.Errorf("title = %q", got)
	}
}

func TestSwissregSearch_PageSizeClamp(t *testing.T) {
	var gotBody swissregReqBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"totalItems":0,"results":[]}`))
	}))
	defer srv.Close()
	defer pointSwissregAt(t, srv)()

	c := newTestClient(t)
	if _, err := c.SwissregSearch(SwissregQuery{Target: "chmarke", PageSize: 999}); err != nil {
		t.Fatal(err)
	}
	if gotBody.PageSize != SwissregMaxPageSize {
		t.Errorf("pageSize not clamped: %d, want %d", gotBody.PageSize, SwissregMaxPageSize)
	}

	if _, err := c.SwissregSearch(SwissregQuery{Target: "chmarke", PageSize: 0}); err != nil {
		t.Fatal(err)
	}
	if gotBody.PageSize != 16 {
		t.Errorf("default pageSize = %d, want 16", gotBody.PageSize)
	}
}

func TestSwissregDetail_BuildsURN(t *testing.T) {
	var gotBody swissregReqBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"totalItems":1,"results":[{"id":["urn:ige:schutztitel:chmarke:1"],"titel__type_text":["X"]}]}`))
	}))
	defer srv.Close()
	defer pointSwissregAt(t, srv)()

	c := newTestClient(t)

	// Bare id → URN is constructed.
	if _, err := c.SwissregDetail("chmarke", "12345"); err != nil {
		t.Fatal(err)
	}
	got := gotBody.Filters["id"]
	if len(got) != 1 || got[0] != "urn:ige:schutztitel:chmarke:12345" {
		t.Errorf("id filter = %v", got)
	}

	// Full URN → passed through untouched.
	if _, err := c.SwissregDetail("chmarke", "urn:ige:schutztitel:chmarke:99"); err != nil {
		t.Fatal(err)
	}
	got = gotBody.Filters["id"]
	if len(got) != 1 || got[0] != "urn:ige:schutztitel:chmarke:99" {
		t.Errorf("urn not preserved: %v", got)
	}
}

func TestSwissregDetail_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalItems":0,"results":[]}`))
	}))
	defer srv.Close()
	defer pointSwissregAt(t, srv)()

	c := newTestClient(t)
	r, err := c.SwissregDetail("chmarke", "nope")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r != nil {
		t.Errorf("expected nil, got %+v", r)
	}
}

func TestSwissregSearch_UnknownTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	defer pointSwissregAt(t, srv)()

	c := newTestClient(t)
	_, err := c.SwissregSearch(SwissregQuery{Target: "bogus", SearchString: "x", PageSize: 1})
	if err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("expected unknown-target error, got %v", err)
	}
}

func TestSwissregImageURL(t *testing.T) {
	hash := "CC750DB3BB36DA96091BC9ABA18F485364EF6F37"
	got := SwissregImageURL(hash)
	want := "https://www.swissreg.ch/database/resources/ds/image/urn:ige:img:" + hash
	if got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
	if SwissregImageURL("") != "" {
		t.Errorf("empty hash should yield empty URL")
	}
}

func TestSwissregResult_First(t *testing.T) {
	r := SwissregResult{
		"titel__type_text": []string{"Foo", "Bar"},
		"empty":            []string{},
	}
	if r.First("titel__type_text") != "Foo" {
		t.Errorf("First wrong")
	}
	if r.First("empty") != "" {
		t.Errorf("empty array should yield empty string")
	}
	if r.First("absent") != "" {
		t.Errorf("absent key should yield empty string")
	}
}
