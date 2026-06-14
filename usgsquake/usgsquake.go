// Package usgsquake is the library behind the quake command line:
// the HTTP client, request shaping, and typed data models for the USGS
// FDSNWS Earthquake Event Web Service (earthquake.usgs.gov/fdsnws/event/1).
//
// The USGS API is free and requires no API key. It returns seismic events
// worldwide in GeoJSON format with magnitude, location, depth, and metadata.
package usgsquake

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Host is the API host this client talks to.
const Host = "earthquake.usgs.gov"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://earthquake.usgs.gov/fdsnws/event/1",
		UserAgent: "usgsquake-cli/0.1.0 (github.com/tamnd/usgsquake-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to the USGS earthquake API over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// ListParams holds optional filters for a list query.
type ListParams struct {
	MinMag float64
	MaxMag float64
	Start  string
	End    string
	Limit  int
}

// List queries /query for earthquakes matching the given params.
func (c *Client) List(ctx context.Context, p ListParams) ([]Earthquake, error) {
	q := url.Values{}
	q.Set("format", "geojson")
	if p.MinMag > 0 {
		q.Set("minmagnitude", strconv.FormatFloat(p.MinMag, 'f', -1, 64))
	}
	if p.MaxMag > 0 {
		q.Set("maxmagnitude", strconv.FormatFloat(p.MaxMag, 'f', -1, 64))
	}
	if p.Start != "" {
		q.Set("starttime", p.Start)
	}
	if p.End != "" {
		q.Set("endtime", p.End)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	body, err := c.get(ctx, c.cfg.BaseURL+"/query?"+q.Encode())
	if err != nil {
		return nil, err
	}
	return parseFeatureCollection(body)
}

// Nearby queries /query for earthquakes near a point.
func (c *Client) Nearby(ctx context.Context, lat, lon, radiusKm, minMag float64, limit int) ([]Earthquake, error) {
	q := url.Values{}
	q.Set("format", "geojson")
	q.Set("latitude", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', -1, 64))
	q.Set("maxradiuskm", strconv.FormatFloat(radiusKm, 'f', -1, 64))
	if minMag > 0 {
		q.Set("minmagnitude", strconv.FormatFloat(minMag, 'f', -1, 64))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	body, err := c.get(ctx, c.cfg.BaseURL+"/query?"+q.Encode())
	if err != nil {
		return nil, err
	}
	return parseFeatureCollection(body)
}

// CountParams holds filters for a count query.
type CountParams struct {
	MinMag float64
	Start  string
	End    string
}

// Count queries /count for the number of earthquakes matching params.
func (c *Client) Count(ctx context.Context, p CountParams) (*Count, error) {
	q := url.Values{}
	q.Set("format", "json")
	if p.MinMag > 0 {
		q.Set("minmagnitude", strconv.FormatFloat(p.MinMag, 'f', -1, 64))
	}
	if p.Start != "" {
		q.Set("starttime", p.Start)
	}
	if p.End != "" {
		q.Set("endtime", p.End)
	}
	body, err := c.get(ctx, c.cfg.BaseURL+"/count?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var ct Count
	if err := json.Unmarshal(body, &ct); err != nil {
		return nil, fmt.Errorf("decode count: %w", err)
	}
	return &ct, nil
}

// Get fetches a single earthquake event by USGS event ID.
func (c *Client) Get(ctx context.Context, eventID string) (*Earthquake, error) {
	q := url.Values{}
	q.Set("format", "geojson")
	q.Set("eventid", eventID)
	body, err := c.get(ctx, c.cfg.BaseURL+"/query?"+q.Encode())
	if err != nil {
		return nil, err
	}
	// Single event response is a Feature, not FeatureCollection
	var feature rawFeature
	if err := json.Unmarshal(body, &feature); err != nil {
		return nil, fmt.Errorf("decode feature: %w", err)
	}
	if feature.ID != "" {
		return convertFeature(feature), nil
	}
	// Might also come as a FeatureCollection with one item
	eqs, err := parseFeatureCollection(body)
	if err != nil {
		return nil, err
	}
	if len(eqs) == 0 {
		return nil, fmt.Errorf("event %q not found", eventID)
	}
	return &eqs[0], nil
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}

// --- raw API response types ---

type rawFeatureCollection struct {
	Type     string       `json:"type"`
	Features []rawFeature `json:"features"`
}

type rawFeature struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Properties rawProperties  `json:"properties"`
	Geometry   rawGeometry    `json:"geometry"`
}

type rawProperties struct {
	Mag    float64 `json:"mag"`
	Place  string  `json:"place"`
	Time   int64   `json:"time"`
	Updated int64  `json:"updated"`
	URL    string  `json:"url"`
	Title  string  `json:"title"`
	Status string  `json:"status"`
	Tsunami int    `json:"tsunami"`
	Sig    int     `json:"sig"`
	Type   string  `json:"type"`
}

type rawGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

func parseFeatureCollection(body []byte) ([]Earthquake, error) {
	var fc rawFeatureCollection
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("decode feature collection: %w", err)
	}
	out := make([]Earthquake, 0, len(fc.Features))
	for _, f := range fc.Features {
		out = append(out, *convertFeature(f))
	}
	return out, nil
}

func convertFeature(f rawFeature) *Earthquake {
	eq := &Earthquake{
		ID:           f.ID,
		Place:        f.Properties.Place,
		Title:        f.Properties.Title,
		Status:       f.Properties.Status,
		Type:         f.Properties.Type,
		Magnitude:    f.Properties.Mag,
		Time:         f.Properties.Time,
		Updated:      f.Properties.Updated,
		Tsunami:      f.Properties.Tsunami,
		Significance: f.Properties.Sig,
		URL:          f.Properties.URL,
	}
	// coordinates = [lon, lat, depth]
	if len(f.Geometry.Coordinates) >= 3 {
		eq.Lon = f.Geometry.Coordinates[0]
		eq.Lat = f.Geometry.Coordinates[1]
		eq.Depth = f.Geometry.Coordinates[2]
	} else if len(f.Geometry.Coordinates) == 2 {
		eq.Lon = f.Geometry.Coordinates[0]
		eq.Lat = f.Geometry.Coordinates[1]
	}
	return eq
}
