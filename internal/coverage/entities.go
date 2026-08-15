package coverage

import (
	"html"
	"strings"
)

// HasEncodedEntities reports whether a string still carries HTML entity
// escapes — "Annabel&#39;s" rather than "Annabel's".
//
// Some place names reach the SEL with their source page's HTML escaping intact,
// so the stored value is the escaped text rather than the text itself. This is
// a storage defect, not a rendering one: every consumer of the API inherits it,
// and any consumer that renders with textContent (correctly, to avoid injection)
// will display the raw entity to a reader.
//
// Counted as a quality signal rather than silently repaired, so the rate stays
// visible — see Report.Quality.EncodedEntities.
func HasEncodedEntities(s string) bool {
	return s != html.UnescapeString(s)
}

// DecodeEntities renders a stored name readable.
//
// Applied only at the aggregation boundary, and only for display. It does not
// make the stored data correct; it stops the dashboard from repeating the
// defect back at the operator while still counting it.
func DecodeEntities(s string) string {
	return strings.TrimSpace(html.UnescapeString(s))
}
