// Package coverage derives coverage and gap metrics from a SEL node's public
// event data.
//
// The emphasis is deliberately on absence rather than totals. Totals only ever
// go up and say little; a venue that has stopped appearing, or a week with no
// events, is the signal an operator can act on.
package coverage

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/Togather-Foundation/dashboard/internal/sel"
)

// Report is the payload the coverage panel renders.
type Report struct {
	GeneratedAt  time.Time  `json:"generatedAt"`
	Horizon      int        `json:"horizonDays"`
	TotalEvents  int        `json:"totalEvents"`
	ActiveVenues int        `json:"activeVenues"`
	Days         []DayCount `json:"days"`
	QuietDays    []string   `json:"quietDays"`
	TopVenues    []Venue    `json:"topVenues"`
	QuietVenues  []Venue    `json:"quietVenues"`
	Quality      Quality    `json:"quality"`
	Truncated    bool       `json:"truncated"`
}

type DayCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type Venue struct {
	Name     string `json:"name"`
	Count    int    `json:"count"`
	LastSeen string `json:"lastSeen,omitempty"`
	HasGeo   bool   `json:"hasGeo"`
	PlaceID  string `json:"placeId,omitempty"`
}

// Quality counts data defects that are invisible in a total but obvious in a
// list. Each corresponds to a known upstream issue.
type Quality struct {
	MissingGeo         int `json:"missingGeo"` // venue cannot be mapped
	MissingDescription int `json:"missingDescription"`
	UnknownStartTime   int `json:"unknownStartTime"` // date-only source, stored as local midnight
	DuplicatedName     int `json:"duplicatedName"`   // server#23
	EncodedEntities    int `json:"encodedEntities"`  // name stored with HTML escapes intact
}

const (
	pageSize = 200
	maxPages = 12 // hard ceiling; see Truncated
)

// Build walks the public events endpoint over the given horizon and derives
// the report.
//
// This paginates because the API exposes no aggregate or count capability
// (Togather-Foundation/server#24). If that lands, most of this becomes a query.
func Build(ctx context.Context, c *sel.Client, horizonDays int, loc *time.Location) (*Report, error) {
	now := time.Now().In(loc)
	start := now.Format("2006-01-02")
	end := now.AddDate(0, 0, horizonDays).Format("2006-01-02")

	rep := &Report{
		GeneratedAt: now,
		Horizon:     horizonDays,
		Days:        []DayCount{},
		QuietDays:   []string{},
		TopVenues:   []Venue{},
		QuietVenues: []Venue{},
	}

	perDay := map[string]int{}
	venues := map[string]*Venue{}

	cursor := ""
	for page := 0; page < maxPages; page++ {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("startDate", start)
		q.Set("endDate", end)
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		res, err := c.Events(ctx, q)
		if err != nil {
			return nil, err
		}

		for _, ev := range res.Items {
			t, err := time.Parse(time.RFC3339, ev.StartDate)
			if err != nil {
				continue
			}
			local := t.In(loc)
			day := local.Format("2006-01-02")
			perDay[day]++
			rep.TotalEvents++

			if ev.Description == "" {
				rep.Quality.MissingDescription++
			}
			// A start of exactly local midnight means the source carried a date
			// but no time, not an event that begins at 00:00.
			if local.Hour() == 0 && local.Minute() == 0 {
				rep.Quality.UnknownStartTime++
			}

			if ev.Location == nil || ev.Location.Name == "" {
				continue
			}
			name := CleanVenueName(ev.Location.Name)
			if name != ev.Location.Name {
				rep.Quality.DuplicatedName++
			}
			if HasEncodedEntities(name) {
				rep.Quality.EncodedEntities++
				name = DecodeEntities(name)
			}

			v, ok := venues[name]
			if !ok {
				v = &Venue{Name: name, PlaceID: ev.Location.ID}
				venues[name] = v
			}
			v.Count++
			if ev.Location.Geo != nil {
				v.HasGeo = true
			}
			if day > v.LastSeen {
				v.LastSeen = day
			}
		}

		cursor = res.NextCursor
		// next_cursor is returned even on a final short page, so a full page is
		// also required before assuming more exists.
		if cursor == "" || len(res.Items) < pageSize {
			cursor = ""
			break
		}
		if page == maxPages-1 {
			rep.Truncated = true
		}
	}

	// Dense day series including zeros — the zeros are the point.
	for i := 0; i <= horizonDays; i++ {
		d := now.AddDate(0, 0, i).Format("2006-01-02")
		n := perDay[d]
		rep.Days = append(rep.Days, DayCount{Date: d, Count: n})
		if n == 0 {
			rep.QuietDays = append(rep.QuietDays, d)
		}
	}

	all := make([]Venue, 0, len(venues))
	for _, v := range venues {
		if !v.HasGeo {
			rep.Quality.MissingGeo++
		}
		all = append(all, *v)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Count != all[j].Count {
			return all[i].Count > all[j].Count
		}
		return all[i].Name < all[j].Name
	})

	rep.ActiveVenues = len(all)

	if len(all) > 12 {
		rep.TopVenues = all[:12]
	} else {
		rep.TopVenues = all
	}

	// Quiet venues need the places collection: a venue with no upcoming events
	// does not appear in the event feed at all, which is exactly why it matters.
	quiet, err := quietVenues(ctx, c, venues)
	if err == nil {
		rep.QuietVenues = quiet
	}

	return rep, nil
}

// quietVenues lists known places that have no events in the horizon.
func quietVenues(ctx context.Context, c *sel.Client, active map[string]*Venue) ([]Venue, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(pageSize))

	res, err := c.Places(ctx, q)
	if err != nil {
		return nil, err
	}

	var out []Venue
	for _, p := range res.Items {
		name := DecodeEntities(CleanVenueName(p.Name))
		if _, busy := active[name]; busy {
			continue
		}
		out = append(out, Venue{
			Name:    name,
			Count:   0,
			HasGeo:  p.Geo != nil,
			PlaceID: p.ID,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) > 25 {
		out = out[:25]
	}
	return out, nil
}
