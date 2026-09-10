package sites

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoad(t *testing.T) {
	file := filepath.Join(t.TempDir(), "urls.json")
	content := `["https://example.com/$USERNAME", "", "   "]`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(file)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []Site{{
		URL:        "https://example.com/$USERNAME",
		ErrorType:  Types{StatusCode},
		NoRedirect: true,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadObjectsAndStrings(t *testing.T) {
	file := filepath.Join(t.TempDir(), "urls.json")
	content := `[
		"https://plain.example/$USERNAME",
		{
			"name": "Example",
			"url": "https://example.com/u/$USERNAME",
			"urlProbe": "https://example.com/api/$USERNAME",
			"errorType": "message",
			"errorMsg": "not found",
			"regexCheck": "^[a-z]+$",
			"request_method": "POST",
			"request_payload": {"name": "$USERNAME"},
			"errorCode": 410
		},
		{
			"url": "https://combo.example/$USERNAME",
			"errorType": ["message", "status_code"],
			"errorMsg": ["gone", "missing"],
			"errorCode": [404, 410]
		}
	]`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(file)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if !got[0].NoRedirect || got[0].URL != "https://plain.example/$USERNAME" {
		t.Errorf("string entry = %#v", got[0])
	}
	if got[1].Name != "Example" || got[1].RequestMethod != "POST" || got[1].ErrorCode[0] != 410 {
		t.Errorf("object entry = %#v", got[1])
	}
	if string(got[1].RequestPayload) != `{"name": "$USERNAME"}` {
		t.Errorf("payload = %s", got[1].RequestPayload)
	}
	if !got[2].ErrorType.Has(Message) || !got[2].ErrorType.Has(StatusCode) {
		t.Errorf("combined errorType = %v", got[2].ErrorType)
	}
	if !reflect.DeepEqual(got[2].ErrorMsg, []string{"gone", "missing"}) {
		t.Errorf("errorMsg = %v", got[2].ErrorMsg)
	}
}

func TestLoadProjectURLs(t *testing.T) {
	list, err := Load(filepath.Join("..", "..", "urls.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 1000 {
		t.Fatalf("len = %d, want a full catalog", len(list))
	}
	var named int
	for _, s := range list {
		if s.Name != "" {
			named++
		}
	}
	if named < 500 {
		t.Fatalf("named sites = %d, want Sherlock overlays", named)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	file := filepath.Join(t.TempDir(), "urls.json")
	if err := os.WriteFile(file, []byte(`{"not": "an array"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil {
		t.Error("expected error for non-array JSON, got nil")
	}
}

func TestTypesRoundTrip(t *testing.T) {
	var t1 Types
	if err := json.Unmarshal([]byte(`"message"`), &t1); err != nil {
		t.Fatal(err)
	}
	if !t1.Has(Message) || len(t1) != 1 {
		t.Errorf("string unmarshal: %v", t1)
	}
	raw, err := json.Marshal(t1)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `"message"` {
		t.Errorf("marshal one value = %s", raw)
	}
}
