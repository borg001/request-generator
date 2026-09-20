package renderer

import "testing"

// A wide screen can ask a gallery strip for a different number of thumbnails,
// and only a gallery has a strip to ask it of.
func TestThumbLimitWideBelongsToAGallery(t *testing.T) {
	gallery := DisplayComponent{ID: "gallery", Type: DisplayMediaGallery, ThumbLimit: 4, ThumbLimitWide: 3}
	if err := gallery.Validate(); err != nil {
		t.Fatalf("a gallery may carry a wide thumb limit: %v", err)
	}
	negative := DisplayComponent{ID: "gallery", Type: DisplayMediaGallery, ThumbLimitWide: -1}
	if err := negative.Validate(); err == nil {
		t.Fatal("a negative wide thumb limit must be refused")
	}
	text := DisplayComponent{ID: "about", Type: DisplayText, ThumbLimitWide: 3}
	if err := text.Validate(); err == nil {
		t.Fatal("a wide thumb limit on a component without a strip must be refused")
	}
}
