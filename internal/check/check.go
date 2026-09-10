// Package check performs username lookups against site URL templates.
package check

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/isanjaymenon/ufo/internal/sites"
)

// Placeholder is the token replaced by the username in URL templates.
const Placeholder = "$USERNAME"

const maxBody = 1 << 20 // 1 MiB

// userAgents holds user agent strings used to randomize request headers.
var userAgents = []string{
	"Mozilla/5.0 (iPhone14,3; U; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/602.1.50 (KHTML, like Gecko) Version/10.0 Mobile/19A346 Safari/602.1",
	"Mozilla/5.0 (Linux; Android 13; M2101K6G) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; Pixel 7 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; SM-S908B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; SM-S908U) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (X11; CrOS x86_64 8172.45.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.64 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.107 Safari/537.36",
}

// wafFingerprints are response snippets that mean a bot challenge, not a profile.
var wafFingerprints = []string{
	`.loading-spinner{visibility:hidden}body.no-js .challenge-running{display:none}`,
	`<span id="challenge-error-text">`,
	`AwsWafIntegration.forceRefreshToken`,
	`{return l.onPageView}}),Object.defineProperty(r,"perimeterxIdentifiers",{enumerable:`,
}

// Runner checks site URL templates for a username concurrently.
type Runner struct {
	// Username is substituted for Placeholder in each URL template.
	Username string
	// Concurrency limits the number of parallel requests (minimum 1).
	Concurrency int
	// Timeout is the per-request timeout used when Client is nil.
	Timeout time.Duration
	// Client overrides the default HTTP client.
	Client *http.Client
	// OnError, when set, reports request failures.
	OnError func(err error)
	// DisableDelay skips the random pause before each request.
	DisableDelay bool
}

// Run checks each site and invokes onFound — always from a single
// goroutine, so it does not need to be safe for concurrent use — with the
// substituted profile URL of every site that appears to have the account.
func (r *Runner) Run(ctx context.Context, list []sites.Site, onFound func(url string)) error {
	concurrency := r.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: r.Timeout}
	}

	found := make(chan string)
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for url := range found {
			onFound(url)
		}
	}()

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

loop:
	for _, site := range list {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break loop
		}
		wg.Add(1)
		go func(site sites.Site) {
			defer wg.Done()
			defer func() { <-sem }()
			r.checkOne(ctx, client, site, found)
		}(site)
	}
	wg.Wait()
	close(found)
	<-consumerDone

	return ctx.Err()
}

func (r *Runner) checkOne(ctx context.Context, client *http.Client, site sites.Site, found chan<- string) {
	username := strings.ReplaceAll(r.Username, " ", "%20")
	profile := interpolate(site.URL, username)

	if site.RegexCheck != "" {
		re, err := regexp.Compile(site.RegexCheck)
		if err != nil {
			r.reportError(fmt.Errorf("%s: regexCheck: %w", profile, err))
			return
		}
		if !re.MatchString(r.Username) {
			return
		}
	}

	if !r.DisableDelay {
		select {
		case <-time.After(randomDelay()):
		case <-ctx.Done():
			return
		}
	}

	probe := interpolate(site.ProbeURL(), username)
	resp, body, err := r.do(ctx, client, site, probe, username)
	if err != nil {
		r.reportError(fmt.Errorf("%s: %w", probe, err))
		return
	}
	if accountExists(site, resp, body) {
		found <- profile
	}
}

func (r *Runner) do(ctx context.Context, client *http.Client, site sites.Site, probe, username string) (*http.Response, string, error) {
	method := probeMethod(site)
	resp, body, err := r.roundTrip(ctx, client, site, method, probe, username)
	if err != nil {
		return nil, "", err
	}
	if method == http.MethodHead && resp.StatusCode == http.StatusMethodNotAllowed {
		resp, body, err = r.roundTrip(ctx, client, site, http.MethodGet, probe, username)
	}
	return resp, body, err
}

func (r *Runner) roundTrip(ctx context.Context, client *http.Client, site sites.Site, method, probe, username string) (*http.Response, string, error) {
	c := client
	if !site.FollowRedirects() {
		clone := *client
		clone.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		c = &clone
	}

	var bodyReader io.Reader
	if len(site.RequestPayload) > 0 {
		bodyReader = strings.NewReader(interpolate(string(site.RequestPayload), username))
	}

	req, err := http.NewRequestWithContext(ctx, method, probe, bodyReader)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", randomUserAgent())
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range site.Headers {
		req.Header.Set(k, interpolate(v, username))
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp, "", err
	}
	return resp, string(raw), nil
}

func probeMethod(site sites.Site) string {
	if m := strings.ToUpper(strings.TrimSpace(site.RequestMethod)); m != "" {
		return m
	}
	if len(site.RequestPayload) > 0 {
		return http.MethodPost
	}
	det := site.Detection()
	if det.Has(sites.Message) || det.Has(sites.ResponseURL) {
		return http.MethodGet
	}
	return http.MethodHead
}

func accountExists(site sites.Site, resp *http.Response, body string) bool {
	if resp == nil {
		return false
	}
	if isWAF(body) {
		return false
	}

	det := site.Detection()
	for _, t := range det {
		switch t {
		case sites.Message, sites.StatusCode, sites.ResponseURL:
		default:
			return false
		}
	}

	available := false

	if det.Has(sites.Message) {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			available = true
		} else if containsAny(body, site.ErrorMsg) {
			available = true
		}
	}

	if det.Has(sites.StatusCode) && !available {
		if inCodes(resp.StatusCode, site.ErrorCode) || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			available = true
		}
	}

	if det.Has(sites.ResponseURL) && !available {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			available = true
		}
	}

	return !available
}

func isWAF(body string) bool {
	for _, fp := range wafFingerprints {
		if strings.Contains(body, fp) {
			return true
		}
	}
	return false
}

func containsAny(body string, msgs []string) bool {
	for _, msg := range msgs {
		if msg != "" && strings.Contains(body, msg) {
			return true
		}
	}
	return false
}

func inCodes(status int, codes []int) bool {
	for _, c := range codes {
		if status == c {
			return true
		}
	}
	return false
}

func interpolate(template, username string) string {
	s := strings.ReplaceAll(template, Placeholder, username)
	return strings.ReplaceAll(s, "{}", username)
}

func (r *Runner) reportError(err error) {
	if r.OnError != nil {
		r.OnError(err)
	}
}

// randomDelay returns a duration between 100ms and 1s to mimic human behavior.
func randomDelay() time.Duration {
	return time.Duration(100+rand.IntN(900)) * time.Millisecond
}

func randomUserAgent() string {
	return userAgents[rand.IntN(len(userAgents))]
}
