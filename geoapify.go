package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Geoapify geocoding integration. Converts a postal address into latitude /
// longitude via the Geoapify Geocoding API so the dynamic world-map overview
// can place each location at its real geographic position.
//
// Endpoint: https://api.geoapify.com/v1/geocode/search?text=<addr>&apiKey=<key>
// The API key is stored in the hidden geoconfig bucket (it is a secret and must
// not appear in the visible config_general table). Syncing is always manual.

const geoapifyEndpoint = "https://api.geoapify.com/v1/geocode/search"

// geoEnabled reports whether the Geoapify geocoding integration is switched on.
// It defaults to enabled so existing installs keep working after an upgrade; the
// admin toggle only stores the value "0" to disable it.
func (app *App) geoEnabled() bool {
	return app.db.GetGeoSetting("geoEnabled") != "0"
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

// geocodeAddress resolves a free-form address to coordinates using the given
// API key. Returns the best (first) match, including the ISO country code, city
// and IANA timezone name when the provider supplies them. An empty key or
// address is an error.
func geocodeAddress(ctx context.Context, apiKey, text string) (lat, lon float64, formatted, country, city, timezone string, err error) {
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

// geoAddressText turns a stored map address (newlines encoded as <br/>) into a
// single-line query string suitable for geocoding.
func geoAddressText(stored string) string {
	plain := stripBR(stored)
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

// GeoSyncMapResult is one location's outcome during a batch geocode run.
type GeoSyncMapResult struct {
	Mapname   string  `json:"mapname"`
	Address   string  `json:"address"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Formatted string  `json:"formatted"`
	Status    string  `json:"status"` // "ok" | "skipped" | "error"
	Message   string  `json:"message"`
}

// GeoSyncResult summarises a batch geocode run.
type GeoSyncResult struct {
	Total      int                `json:"total"`
	Updated    int                `json:"updated"`
	Skipped    int                `json:"skipped"`
	Failed     int                `json:"failed"`
	Results    []GeoSyncMapResult `json:"results"`
	UsageMonth string             `json:"usageMonth"` // "2006-01" the count below belongs to
	UsageCount int                `json:"usageCount"` // total Geoapify requests this month (estimate)
}

// RunGeoapifySync geocodes every map that has an address and stores the
// resulting lat/lon back onto the map record. The "overview" map is skipped
// (it is the world map itself, not a physical location). This is only ever
// invoked manually from the admin Sync panel — there is no scheduler. Progress
// is reported through prog so the admin panel can show a live progress bar.
func (app *App) RunGeoapifySync(prog *syncProgress) GeoSyncResult {
	var res GeoSyncResult
	apiKey := app.db.GetGeoSetting("geoapifyApiKey")

	maps, _ := app.db.ListMaps()
	// Count the locations we will actually attempt (everything but "overview")
	// so the progress bar is determinate.
	if prog != nil {
		total := 0
		for _, m := range maps {
			if m.Mapname != "overview" {
				total++
			}
		}
		prog.beginPhase(total, "Geocoding locations…")
	}
	for _, m := range maps {
		if m.Mapname == "overview" {
			continue
		}
		res.Total++
		addr := geoAddressText(m.Address)
		row := GeoSyncMapResult{Mapname: m.Mapname, Address: addr}
		if addr == "" {
			row.Status = "skipped"
			row.Message = "no address set"
			res.Skipped++
			res.Results = append(res.Results, row)
			if prog != nil {
				prog.step("")
				prog.logf("%s: skipped (no address)", m.Mapname)
			}
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		lat, lon, formatted, _, _, _, err := geocodeAddress(ctx, apiKey, addr)
		cancel()
		// Every attempt that reaches the API consumes one request/credit,
		// regardless of whether a match was found, so count it here.
		_, _, _ = app.db.IncrGeoUsage(1)
		if err != nil {
			row.Status = "error"
			row.Message = err.Error()
			res.Failed++
			res.Results = append(res.Results, row)
			if prog != nil {
				prog.step("")
				prog.logf("%s: failed (%s)", m.Mapname, err.Error())
			}
			continue
		}

		m.Lat = lat
		m.Lon = lon
		if err := app.db.PutMap(m); err != nil {
			row.Status = "error"
			row.Message = "could not save: " + err.Error()
			res.Failed++
			res.Results = append(res.Results, row)
			if prog != nil {
				prog.step("")
				prog.logf("%s: could not save (%s)", m.Mapname, err.Error())
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
			prog.step("")
			prog.logf("%s: %.5f, %.5f", m.Mapname, lat, lon)
		}
		// Be polite to the API between requests.
		time.Sleep(120 * time.Millisecond)
	}
	res.UsageMonth, res.UsageCount = app.db.GetGeoUsage()
	return res
}
