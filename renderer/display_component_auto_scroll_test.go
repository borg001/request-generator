package renderer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDisplayComponentAutoScrollIsSentOnlyWhenAsked(t *testing.T) {
	plain, err := json.Marshal(DisplayComponent{ID: "rail"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "auto_scroll") {
		t.Fatalf("a still strip carries auto_scroll: %s", plain)
	}
	moving, err := json.Marshal(DisplayComponent{ID: "rail", AutoScroll: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(moving), `"auto_scroll":true`) {
		t.Fatalf("a moving strip does not say so: %s", moving)
	}
}
