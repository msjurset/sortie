package actionhelp

import (
	"testing"
)

var expectedActions = []string{
	"move", "copy", "rename", "delete", "compress",
	"extract", "symlink", "chmod", "checksum",
	"exec", "notify",
	"convert", "resize", "watermark", "ocr",
	"encrypt", "decrypt",
	"upload", "tag",
	"open", "deduplicate", "unquarantine",
}

func TestAllActionsRegistered(t *testing.T) {
	for _, name := range expectedActions {
		if _, ok := Get(name); !ok {
			t.Errorf("action %q not registered in help registry", name)
		}
	}
}

func TestRegistryCount(t *testing.T) {
	if got := len(List()); got != 22 {
		t.Errorf("List() returned %d actions, want 22", got)
	}
}

func TestGetUnknown(t *testing.T) {
	if _, ok := Get("nonexistent"); ok {
		t.Error("Get(nonexistent) should return false")
	}
}

func TestHelpHasRequiredContent(t *testing.T) {
	for _, name := range expectedActions {
		h, ok := Get(name)
		if !ok {
			continue
		}
		if h.Name == "" {
			t.Errorf("%s: Name is empty", name)
		}
		if h.Description == "" {
			t.Errorf("%s: Description is empty", name)
		}
		if h.Example == "" {
			t.Errorf("%s: Example is empty", name)
		}
	}
}

func TestFormatOutput(t *testing.T) {
	h, _ := Get("move")
	output := Format(h)

	if output == "" {
		t.Fatal("Format() returned empty string")
	}

	checks := []string{"move", "REQUIRED FIELDS", "dest", "EXAMPLE", "USEFUL FOR"}
	for _, check := range checks {
		found := false
		for i := 0; i < len(output)-len(check)+1; i++ {
			if output[i:i+len(check)] == check {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Format() output missing %q", check)
		}
	}
}

func TestListSorted(t *testing.T) {
	list := List()
	for i := 1; i < len(list); i++ {
		if list[i].Name < list[i-1].Name {
			t.Errorf("List() not sorted: %q before %q", list[i-1].Name, list[i].Name)
		}
	}
}
