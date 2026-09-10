// Package sites loads lists of site URL templates used for username checks.
package sites

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Detection methods for whether a username exists on a site.
const (
	StatusCode  = "status_code"
	Message     = "message"
	ResponseURL = "response_url"
)

// Site describes how to probe one site for a username.
type Site struct {
	// Name is an optional human-readable site name.
	Name string `json:"name,omitempty"`
	// URL is the profile URL template. $USERNAME is replaced at check time.
	URL string `json:"url"`
	// URLProbe, if set, is requested instead of URL when detecting existence.
	URLProbe string `json:"urlProbe,omitempty"`
	// ErrorType selects how a response is classified. Defaults to status_code.
	ErrorType Types `json:"errorType,omitempty"`
	// ErrorMsg is the substring(s) that mean "username not found" for message detection.
	ErrorMsg []string `json:"errorMsg,omitempty"`
	// ErrorCode is the HTTP status(es) that mean "username not found" for status_code detection.
	ErrorCode []int `json:"errorCode,omitempty"`
	// RegexCheck, if set, is a pattern the username must match or the site is skipped.
	RegexCheck string `json:"regexCheck,omitempty"`
	// Headers are extra HTTP headers sent with the probe.
	Headers map[string]string `json:"headers,omitempty"`
	// RequestMethod overrides GET/HEAD selection (GET, HEAD, POST, PUT).
	RequestMethod string `json:"requestMethod,omitempty"`
	// RequestPayload is an optional JSON body; $USERNAME is interpolated.
	RequestPayload json.RawMessage `json:"requestPayload,omitempty"`
	// NoRedirect disables following HTTP redirects (also implied by response_url).
	NoRedirect bool `json:"noRedirect,omitempty"`
}

// Types is one or more detection methods (JSON string or array).
type Types []string

// Has reports whether t contains typ.
func (t Types) Has(typ string) bool {
	for _, v := range t {
		if v == typ {
			return true
		}
	}
	return false
}

// UnmarshalJSON accepts a string or an array of strings.
func (t *Types) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s != "" {
			*t = Types{s}
		}
		return nil
	}
	var ss []string
	if err := json.Unmarshal(data, &ss); err != nil {
		return err
	}
	*t = ss
	return nil
}

// MarshalJSON writes a string when there is one value, otherwise an array.
func (t Types) MarshalJSON() ([]byte, error) {
	if len(t) == 1 {
		return json.Marshal(t[0])
	}
	return json.Marshal([]string(t))
}

// ProbeURL returns the URL template that should be requested.
func (s Site) ProbeURL() string {
	if strings.TrimSpace(s.URLProbe) != "" {
		return s.URLProbe
	}
	return s.URL
}

// Detection returns the configured methods, defaulting to status_code.
func (s Site) Detection() Types {
	if len(s.ErrorType) == 0 {
		return Types{StatusCode}
	}
	return s.ErrorType
}

// FollowRedirects reports whether the HTTP client should follow redirects.
func (s Site) FollowRedirects() bool {
	if s.NoRedirect || s.Detection().Has(ResponseURL) {
		return false
	}
	return true
}

// Load reads a JSON file containing a JSON array of URL template strings
// and/or site objects. Empty string entries are dropped. Bare strings are
// treated as status_code probes that do not follow redirects.
func Load(filename string) ([]Site, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading URL file: %w", err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parsing %s as a JSON array: %w", filename, err)
	}

	out := make([]Site, 0, len(items))
	for i, raw := range items {
		site, ok, err := parseEntry(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing %s entry %d: %w", filename, i, err)
		}
		if ok {
			out = append(out, site)
		}
	}
	return out, nil
}

func parseEntry(raw json.RawMessage) (Site, bool, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		return Site{}, false, nil
	}

	var s string
	if err := json.Unmarshal(trim, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return Site{}, false, nil
		}
		return Site{
			URL:        s,
			ErrorType:  Types{StatusCode},
			NoRedirect: true,
		}, true, nil
	}

	var aux siteJSON
	if err := json.Unmarshal(trim, &aux); err != nil {
		return Site{}, false, err
	}
	site, err := aux.toSite()
	if err != nil {
		return Site{}, false, err
	}
	if strings.TrimSpace(site.URL) == "" {
		return Site{}, false, nil
	}
	if len(site.ErrorType) == 0 {
		site.ErrorType = Types{StatusCode}
	}
	return site, true, nil
}

// siteJSON accepts both camelCase and Sherlock-style snake_case keys,
// and string-or-array values for errorMsg / errorCode.
type siteJSON struct {
	Name                string            `json:"name"`
	URL                 string            `json:"url"`
	URLProbe            string            `json:"urlProbe"`
	ErrorType           Types             `json:"errorType"`
	ErrorMsg            json.RawMessage   `json:"errorMsg"`
	ErrorCode           json.RawMessage   `json:"errorCode"`
	RegexCheck          string            `json:"regexCheck"`
	Headers             map[string]string `json:"headers"`
	RequestMethod       string            `json:"requestMethod"`
	RequestMethodSnake  string            `json:"request_method"`
	RequestPayload      json.RawMessage   `json:"requestPayload"`
	RequestPayloadSnake json.RawMessage   `json:"request_payload"`
	NoRedirect          bool              `json:"noRedirect"`
}

func (a siteJSON) toSite() (Site, error) {
	msgs, err := unmarshalStrings(a.ErrorMsg)
	if err != nil {
		return Site{}, fmt.Errorf("errorMsg: %w", err)
	}
	codes, err := unmarshalInts(a.ErrorCode)
	if err != nil {
		return Site{}, fmt.Errorf("errorCode: %w", err)
	}
	method := a.RequestMethod
	if method == "" {
		method = a.RequestMethodSnake
	}
	payload := a.RequestPayload
	if len(payload) == 0 {
		payload = a.RequestPayloadSnake
	}
	return Site{
		Name:           a.Name,
		URL:            a.URL,
		URLProbe:       a.URLProbe,
		ErrorType:      a.ErrorType,
		ErrorMsg:       msgs,
		ErrorCode:      codes,
		RegexCheck:     a.RegexCheck,
		Headers:        a.Headers,
		RequestMethod:  method,
		RequestPayload: payload,
		NoRedirect:     a.NoRedirect,
	}, nil
}

func unmarshalStrings(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err != nil {
		return nil, err
	}
	return ss, nil
}

func unmarshalInts(raw json.RawMessage) ([]int, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return []int{n}, nil
	}
	var ns []int
	if err := json.Unmarshal(raw, &ns); err != nil {
		return nil, err
	}
	return ns, nil
}
