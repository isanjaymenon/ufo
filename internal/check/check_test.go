package check

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/isanjaymenon/ufo/internal/sites"
)

func TestRunFindsOnlyHits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/alice" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	found := run(t, []sites.Site{
		{URL: srv.URL + "/" + Placeholder},
		{URL: srv.URL + "/users/" + Placeholder},
	})
	if len(found) != 1 || found[0] != srv.URL+"/alice" {
		t.Errorf("found = %v, want [%s]", found, srv.URL+"/alice")
	}
}

func TestRunContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &Runner{Username: "alice", Concurrency: 2, Timeout: time.Second, DisableDelay: true}
	var found []string
	err := r.Run(ctx, []sites.Site{{URL: srv.URL + "/" + Placeholder, NoRedirect: true}}, func(url string) {
		found = append(found, url)
	})
	if err == nil {
		t.Error("expected context error, got nil")
	}
	if len(found) != 0 {
		t.Errorf("expected no results, got %v", found)
	}
}

func TestStatusCodeTreatsNon2xxAsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok/alice":
			w.WriteHeader(http.StatusOK)
		case "/err/alice":
			w.WriteHeader(http.StatusInternalServerError)
		case "/gone/alice":
			w.WriteHeader(http.StatusGone)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	found := run(t, []sites.Site{
		{URL: srv.URL + "/ok/" + Placeholder, ErrorType: sites.Types{sites.StatusCode}},
		{URL: srv.URL + "/err/" + Placeholder, ErrorType: sites.Types{sites.StatusCode}},
		{URL: srv.URL + "/gone/" + Placeholder, ErrorType: sites.Types{sites.StatusCode}, ErrorCode: []int{http.StatusGone}},
	})
	if len(found) != 1 || !strings.HasSuffix(found[0], "/ok/alice") {
		t.Errorf("found = %v, want only /ok/alice", found)
	}
}

func TestMessageDetection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "/missing/") {
			io.WriteString(w, `{"error":"user not found"}`)
			return
		}
		io.WriteString(w, `{"user":"alice"}`)
	}))
	defer srv.Close()

	site := func(path string) sites.Site {
		return sites.Site{
			URL:       srv.URL + path + Placeholder,
			ErrorType: sites.Types{sites.Message},
			ErrorMsg:  []string{`"error":"user not found"`},
		}
	}
	found := run(t, []sites.Site{site("/u/"), site("/missing/")})
	if len(found) != 1 || !strings.Contains(found[0], "/u/alice") {
		t.Errorf("found = %v, want only /u/alice", found)
	}
}

func TestMessageNon2xxIsNotAHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "Bad Gateway")
	}))
	defer srv.Close()

	found := run(t, []sites.Site{{
		URL:       srv.URL + "/" + Placeholder,
		ErrorType: sites.Types{sites.Message},
		ErrorMsg:  []string{`"valid":true`},
	}})
	if len(found) != 0 {
		t.Errorf("5xx without errorMsg must not count as found, got %v", found)
	}
}

func TestResponseURLDetection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/u/alice" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	defer srv.Close()

	found := run(t, []sites.Site{
		{URL: srv.URL + "/u/" + Placeholder, ErrorType: sites.Types{sites.ResponseURL}},
		{URL: srv.URL + "/missing/" + Placeholder, ErrorType: sites.Types{sites.ResponseURL}},
	})
	if len(found) != 1 || !strings.HasSuffix(found[0], "/u/alice") {
		t.Errorf("found = %v, want only /u/alice", found)
	}
}

func TestRegexCheckSkipsInvalidUsername(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	found := run(t, []sites.Site{{
		URL:        srv.URL + "/" + Placeholder,
		ErrorType:  sites.Types{sites.StatusCode},
		RegexCheck: `^[0-9]+$`,
	}})
	if hit {
		t.Error("request should not be made when username fails regexCheck")
	}
	if len(found) != 0 {
		t.Errorf("found = %v, want none", found)
	}
}

func TestURLProbeReportsProfileURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/alice" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	found := run(t, []sites.Site{{
		URL:       srv.URL + "/u/" + Placeholder,
		URLProbe:  srv.URL + "/api/" + Placeholder,
		ErrorType: sites.Types{sites.StatusCode},
	}})
	if len(found) != 1 || found[0] != srv.URL+"/u/alice" {
		t.Errorf("found = %v, want profile URL", found)
	}
}

func TestHeadMethodNotAllowedFallsBackToGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	found := run(t, []sites.Site{{
		URL:       srv.URL + "/" + Placeholder,
		ErrorType: sites.Types{sites.StatusCode},
	}})
	if len(found) != 1 {
		t.Errorf("found = %v, want a GET fallback hit", found)
	}
}

func TestWAFBodyIsNotAHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `<html><span id="challenge-error-text">Just a moment</span></html>`)
	}))
	defer srv.Close()

	found := run(t, []sites.Site{{
		URL:       srv.URL + "/" + Placeholder,
		ErrorType: sites.Types{sites.Message},
		ErrorMsg:  []string{"no such user"},
	}})
	if len(found) != 0 {
		t.Errorf("WAF challenge must not count as found, got %v", found)
	}
}

func TestRequestPayloadPosted(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":1}`)
	}))
	defer srv.Close()

	found := run(t, []sites.Site{{
		URL:            srv.URL + "/u/" + Placeholder,
		URLProbe:       srv.URL + "/graphql",
		ErrorType:      sites.Types{sites.Message},
		ErrorMsg:       []string{"User not found"},
		RequestPayload: json.RawMessage(`{"name":"$USERNAME"}`),
	}})
	if gotBody != `{"name":"alice"}` {
		t.Errorf("payload = %q, want interpolated username", gotBody)
	}
	if len(found) != 1 {
		t.Errorf("found = %v, want profile URL", found)
	}
}

func TestBareStringSiteDoesNotFollowRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing/alice" {
			http.Redirect(w, r, "/home", http.StatusFound)
			return
		}
		if r.URL.Path == "/home" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	found := run(t, []sites.Site{
		{URL: srv.URL + "/u/" + Placeholder, ErrorType: sites.Types{sites.StatusCode}, NoRedirect: true},
		{URL: srv.URL + "/missing/" + Placeholder, ErrorType: sites.Types{sites.StatusCode}, NoRedirect: true},
	})
	if len(found) != 1 || !strings.HasSuffix(found[0], "/u/alice") {
		t.Errorf("redirect to 200 home must not count as found, got %v", found)
	}
}

func run(t *testing.T, list []sites.Site) []string {
	t.Helper()
	r := &Runner{
		Username:     "alice",
		Concurrency:  4,
		Timeout:      5 * time.Second,
		DisableDelay: true,
	}
	var found []string
	if err := r.Run(context.Background(), list, func(url string) {
		found = append(found, url)
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return found
}
