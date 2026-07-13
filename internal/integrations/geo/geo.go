// Package geo implements the Geoapify geocoding integration. It converts a
// postal address into latitude/longitude via the Geoapify Geocoding API so
// the dynamic world-map overview can place each location at its real
// geographic position.
//
// Endpoint: https://api.geoapify.com/v1/geocode/search?text=<addr>&apiKey=<key>
// The API key is stored in the hidden geoconfig bucket (it is a secret and
// must not appear in the visible config_general table). Syncing is always
// manual.
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"companymaps/internal/progress"
	"companymaps/internal/store"
)

const geoapifyEndpoint = "https://api.geoapify.com/v1/geocode/search"

// Service owns the Geoapify integration state: the store handle plus the
// progress/result pair polled by the admin Sync panel.
type Service struct {
	DB *store.DB

	Prog progress.Progress

	mu     sync.Mutex
	result SyncResult
}

// Enabled reports whether the Geoapify geocoding integration is switched on.
// It defaults to enabled so existing installs keep working after an upgrade;
// the admin toggle only stores the value "0" to disable it.
func (s *Service) Enabled() bool {
	return s.DB.GetGeoSetting("geoEnabled") != "0"
}

// Result returns the most recent completed batch run.
func (s *Service) Result() SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}

// setResult stores the outcome of a completed batch run.
func (s *Service) setResult(r SyncResult) {
	s.mu.Lock()
	s.result = r
	s.mu.Unlock()
}

// geoapifyResponse is the subset of the Geoapify GeoJSON response we consume.
type geoapifyResponse struct {
	Features []struct {
		Properties struct {
			Lat         float64 `json:"lat"`
			Lon         float64 `json:"lon"`
			Formatted   string  `json:"formatted"`
			CountryCode string  `json:"country_code"`
			City        string  `json:"city"`
			Timezone    struct {
				Name string `json:"name"`
			} `json:"timezone"`
		} `json:"properties"`
	} `json:"features"`
}

// GeocodeAddress resolves a free-form address to coordinates using the given
// API key. Returns the best (first) match, including the ISO country code,
// city and IANA timezone name when the provider supplies them. An empty key
// or address is an error.
func GeocodeAddress(ctx context.Context, apiKey, text string) (lat, lon float64, formatted, country, city, timezone string, err error) {
	apiKey = strings.TrimSpace(apiKey)
	text = strings.TrimSpace(text)
	if apiKey == "" {
		return 0, 0, "", "", "", "", fmt.Errorf("no Geoapify API key configured")
	}
	if text == "" {
		return 0, 0, "", "", "", "", fmt.Errorf("address is empty")
	}

	q := url.Values{}
	q.Set("text", text)
	q.Set("limit", "1")
	q.Set("format", "geojson")
	q.Set("apiKey", apiKey)
	reqURL := geoapifyEndpoint + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, "", "", "", "", err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, 0, "", "", "", "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return 0, 0, "", "", "", "", err
	}
	if res.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return 0, 0, "", "", "", "", fmt.Errorf("Geoapify returned HTTP %d: %s", res.StatusCode, msg)
	}

	var parsed geoapifyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, 0, "", "", "", "", fmt.Errorf("could not parse Geoapify response: %v", err)
	}
	if len(parsed.Features) == 0 {
		return 0, 0, "", "", "", "", fmt.Errorf("no match found for address")
	}
	p := parsed.Features[0].Properties
	return p.Lat, p.Lon, p.Formatted, strings.ToLower(p.CountryCode), p.City, p.Timezone.Name, nil
}

// SyncMapResult is one location's outcome during a batch geocode run.
type SyncMapResult struct {
	Mapname   string  `json:"mapname"`
	Address   string  `json:"address"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Formatted string  `json:"formatted"`
	Status    string  `json:"status"` // "ok" | "skipped" | "error"
	Message   string  `json:"message"`
}

// SyncResult summarises a batch geocode run.
type SyncResult struct {
	Total      int             `json:"total"`
	Updated    int             `json:"updated"`
	Skipped    int             `json:"skipped"`
	Failed     int             `json:"failed"`
	Results    []SyncMapResult `json:"results"`
	UsageMonth string          `json:"usageMonth"` // "2006-01" the count below belongs to
	UsageCount int             `json:"usageCount"` // total Geoapify requests this month (estimate)
}

// AddressText turns a stored map address (newlines encoded as <br/>) into a
// single-line query string suitable for geocoding.
func AddressText(stored string) string {
	plain := strings.ReplaceAll(stored, "<br/>", "\n")
	plain = strings.ReplaceAll(plain, "<br>", "\n")
	plain = strings.ReplaceAll(plain, "\r\n", "\n")
	parts := strings.Split(plain, "\n")
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			cleaned = append(cleaned, t)
		}
	}
	return strings.Join(cleaned, ", ")
}

// RunSync geocodes every map that has an address and stores the resulting
// lat/lon back onto the map record. The "overview" map is skipped (it is the
// world map itself, not a physical location). This is only ever invoked
// manually from the admin Sync panel — there is no scheduler. Progress is
// reported through prog so the admin panel can show a live progress bar; the
// result is retained for Result().
func (s *Service) RunSync(prog *progress.Progress) SyncResult {
	var res SyncResult
	apiKey := s.DB.GetGeoSetting("geoapifyApiKey")

	maps, _ := s.DB.ListMaps()
	// Count the locations we will actually attempt (everything but "overview")
	// so the progress bar is determinate.
	if prog != nil {
		total := 0
		for _, m := range maps {
			if m.Mapname != "overview" {
				total++
			}
		}
		prog.BeginPhase(total, "Geocoding locations…")
	}
	for _, m := range maps {
		if m.Mapname == "overview" {
			continue
		}
		res.Total++
		addr := AddressText(m.Address)
		row := SyncMapResult{Mapname: m.Mapname, Address: addr}
		if addr == "" {
			row.Status = "skipped"
			row.Message = "no address set"
			res.Skipped++
			res.Results = append(res.Results, row)
			if prog != nil {
				prog.Step("")
				prog.Logf("%s: skipped (no address)", m.Mapname)
			}
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		lat, lon, formatted, _, _, _, err := GeocodeAddress(ctx, apiKey, addr)
		cancel()
		// Every attempt that reaches the API consumes one request/credit,
		// regardless of whether a match was found, so count it here.
		_, _, _ = s.DB.IncrGeoUsage(1)
		if err != nil {
			row.Status = "error"
			row.Message = err.Error()
			res.Failed++
			res.Results = append(res.Results, row)
			if prog != nil {
				prog.Step("")
				prog.Logf("%s: failed (%s)", m.Mapname, err.Error())
			}
			continue
		}

		m.Lat = lat
		m.Lon = lon
		if err := s.DB.PutMap(m); err != nil {
			row.Status = "error"
			row.Message = "could not save: " + err.Error()
			res.Failed++
			res.Results = append(res.Results, row)
			if prog != nil {
				prog.Step("")
				prog.Logf("%s: could not save (%s)", m.Mapname, err.Error())
			}
			continue
		}
		row.Lat = lat
		row.Lon = lon
		row.Formatted = formatted
		row.Status = "ok"
		res.Updated++
		res.Results = append(res.Results, row)
		if prog != nil {
			prog.Step("")
			prog.Logf("%s: %.5f, %.5f", m.Mapname, lat, lon)
		}
		// Be polite to the API between requests.
		time.Sleep(120 * time.Millisecond)
	}
	res.UsageMonth, res.UsageCount = s.DB.GetGeoUsage()
	s.setResult(res)
	return res
}
