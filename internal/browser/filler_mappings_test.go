package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/drumandbytes/delistd/internal/config"
)

// The selectors and patterns the filler tries, in order, for a full profile.
// The order matters (the first visible match wins), so it is pinned.
func TestFieldMappingsMatchRecorded(t *testing.T) {
	profile := &config.Profile{FirstName: "Jane", MiddleName: "Marie", LastName: "Doe", Email: "j@example.com", Phone: "+1", Address: "1 Main", City: "Riga",
		State: "CA", ZipCode: "LV-1", Country: "LV", DateOfBirth: "1990-01-01"}
	got := NewFormFiller(profile).getFieldMappings()
	file := filepath.Join("testdata", "filler_mappings.json")
	if *updateFiller {
		raw, _ := json.MarshalIndent(got, "", " ")
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var want []FieldMapping
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("field mappings changed:\n got  %+v\n want %+v", got, want)
	}
}

func TestSingleLineKeepsEnterOutOfInputs(t *testing.T) {
	for _, c := range []struct{ tag, in, want string }{
		{"input", "12 Main St\nApt 4", "12 Main St Apt 4"},
		{"input", "a\r\nb\rc", "a b c"},
		{"input", "two  spaces stay", "two  spaces stay"},
		{"textarea", "12 Main St\nApt 4", "12 Main St\nApt 4"},
		{"input", "plain", "plain"},
	} {
		if got := singleLine(c.tag, c.in); got != c.want {
			t.Errorf("singleLine(%q, %q) = %q, want %q", c.tag, c.in, got, c.want)
		}
	}
}
