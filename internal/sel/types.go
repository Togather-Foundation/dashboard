package sel

import "time"

// Types mirror the schemas published at /api/v1/openapi.json on a SEL node.
// Only the fields the dashboard actually reads are modelled; the node emits
// more. Field names follow the wire format, which is inconsistent across
// endpoints — public JSON-LD is camelCase, admin payloads are snake_case.

// ---------- public tier (no credentials) ----------

type EventListResponse struct {
	Items      []Event  `json:"items"`
	NextCursor string   `json:"next_cursor"`
	Warnings   []string `json:"warnings"`
}

type Event struct {
	ID          string   `json:"@id"`
	Type        string   `json:"@type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	StartDate   string   `json:"startDate"`
	EndDate     string   `json:"endDate"`
	Keywords    []string `json:"keywords"`
	Location    *Place   `json:"location"`
}

type Place struct {
	ID      string   `json:"@id"`
	Name    string   `json:"name"`
	Geo     *Geo     `json:"geo"`
	Address *Address `json:"address"`
}

type Geo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Address struct {
	StreetAddress   string `json:"streetAddress"`
	AddressLocality string `json:"addressLocality"`
	PostalCode      string `json:"postalCode"`
	AddressCountry  string `json:"addressCountry"`
}

type PlaceListResponse struct {
	Items      []Place `json:"items"`
	NextCursor string  `json:"next_cursor"`
}

// ---------- admin tier (bearer JWT) ----------

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ScraperSourceSummary is the per-source health record behind
// GET /admin/scraper/sources.
type ScraperSourceSummary struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Enabled             bool   `json:"enabled"`
	ExtractionMethod    string `json:"extraction_method"`
	LastRunStatus       string `json:"last_run_status"`
	LastRunStartedAt    string `json:"last_run_started_at"`
	LastRunCompletedAt  string `json:"last_run_completed_at"`
	LastRunErrorMessage string `json:"last_run_error_message"`
	LastRunEventsFound  int    `json:"last_run_events_found"`
	LastRunEventsNew    int    `json:"last_run_events_new"`
	LastRunEventsDup    int    `json:"last_run_events_dup"`
	LastRunEventsFailed int    `json:"last_run_events_failed"`
}

type ScraperSourcesResponse struct {
	Items []ScraperSourceSummary `json:"items"`
}

// DailyUsageResponse backs GET /admin/reports/daily-usage.
type DailyUsageResponse struct {
	Daily []DailyUsage `json:"daily"`
}

type DailyUsage struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}
