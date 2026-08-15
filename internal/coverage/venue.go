package coverage

import "strings"

// CleanVenueName collapses names stored as "{name}Event location: {name}".
//
// One scraper batch concatenated a field value with its adjacent DOM label and
// no separator, affecting a cohort of Toronto Public Library branches. Tracked
// as Togather-Foundation/server#23. Counting the raw strings would split one
// venue across two keys and understate its activity, so normalise before
// aggregating — and count the occurrences, since the rate is itself a signal.
//
// Delete this once #23 is fixed and backfilled.
func CleanVenueName(name string) string {
	const marker = "Event location:"
	i := strings.Index(name, marker)
	if i <= 0 {
		return name
	}
	head := strings.TrimSpace(name[:i])
	tail := strings.TrimSpace(name[i+len(marker):])
	if head != "" && strings.EqualFold(head, tail) {
		return head
	}
	return name
}
