package coverage

import "testing"

func TestCleanVenueName(t *testing.T) {
	cases := []struct{ in, want string }{
		// The server#23 defect: name concatenated with its own DOM label.
		{"CedarbraeEvent location: Cedarbrae", "Cedarbrae"},
		{"Lillian H. SmithEvent location: Lillian H. Smith", "Lillian H. Smith"},
		{"Bloor/GladstoneEvent location: Bloor/Gladstone", "Bloor/Gladstone"},

		// Untouched when the halves genuinely differ — collapsing these would
		// discard information rather than repair it.
		{"Main HallEvent location: Annex", "Main HallEvent location: Annex"},

		// Ordinary names pass through.
		{"Jazz Bistro", "Jazz Bistro"},
		{"", ""},

		// Leading marker has no name before it; nothing to collapse onto.
		{"Event location: Cedarbrae", "Event location: Cedarbrae"},
	}
	for _, c := range cases {
		if got := CleanVenueName(c.in); got != c.want {
			t.Errorf("CleanVenueName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEntities(t *testing.T) {
	if !HasEncodedEntities("Annabel&#39;s") {
		t.Error("expected HTML-escaped name to be detected")
	}
	if HasEncodedEntities("Annabel's") {
		t.Error("plain name should not be flagged")
	}
	if got := DecodeEntities("Annabel&#39;s,200 Princes&#39; Blvd"); got != "Annabel's,200 Princes' Blvd" {
		t.Errorf("DecodeEntities = %q", got)
	}
	// Ampersands in venue names are common and must survive a round trip.
	if got := DecodeEntities("Auer &amp; Co."); got != "Auer & Co." {
		t.Errorf("DecodeEntities = %q", got)
	}
}
