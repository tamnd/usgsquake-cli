package usgsquake_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/usgsquake-cli/usgsquake"
)

const featureCollectionJSON = `{
  "type": "FeatureCollection",
  "metadata": {"count": 2},
  "features": [
    {
      "type": "Feature",
      "id": "us7000m6al",
      "properties": {
        "mag": 6.1,
        "place": "Auckland Islands, New Zealand",
        "time": 1718000000000,
        "updated": 1718050000000,
        "url": "https://earthquake.usgs.gov/earthquakes/eventpage/us7000m6al",
        "title": "M 6.1 - Auckland Islands, New Zealand",
        "status": "reviewed",
        "tsunami": 0,
        "sig": 572,
        "type": "earthquake"
      },
      "geometry": {
        "type": "Point",
        "coordinates": [174.5, -51.2, 10.0]
      }
    },
    {
      "type": "Feature",
      "id": "us7000abcd",
      "properties": {
        "mag": 5.4,
        "place": "Japan",
        "time": 1718100000000,
        "updated": 1718150000000,
        "url": "https://earthquake.usgs.gov/earthquakes/eventpage/us7000abcd",
        "title": "M 5.4 - Japan",
        "status": "reviewed",
        "tsunami": 1,
        "sig": 440,
        "type": "earthquake"
      },
      "geometry": {
        "type": "Point",
        "coordinates": [139.0, 35.7, 20.0]
      }
    }
  ]
}`

const singleFeatureJSON = `{
  "type": "Feature",
  "id": "us7000m6al",
  "properties": {
    "mag": 6.1,
    "place": "Auckland Islands, New Zealand",
    "time": 1718000000000,
    "updated": 1718050000000,
    "url": "https://earthquake.usgs.gov/earthquakes/eventpage/us7000m6al",
    "title": "M 6.1 - Auckland Islands, New Zealand",
    "status": "reviewed",
    "tsunami": 0,
    "sig": 572,
    "type": "earthquake"
  },
  "geometry": {
    "type": "Point",
    "coordinates": [174.5, -51.2, 10.0]
  }
}`

const countJSON = `{"count": 247}`

func newTestClient(ts *httptest.Server) *usgsquake.Client {
	cfg := usgsquake.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return usgsquake.NewClient(cfg)
}

func TestListParsesFeatureCollection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, featureCollectionJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	eqs, err := c.List(context.Background(), usgsquake.ListParams{MinMag: 4.0, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(eqs) != 2 {
		t.Fatalf("len(eqs) = %d, want 2", len(eqs))
	}
	eq := eqs[0]
	if eq.ID != "us7000m6al" {
		t.Errorf("ID = %q, want us7000m6al", eq.ID)
	}
	if eq.Magnitude != 6.1 {
		t.Errorf("Magnitude = %v, want 6.1", eq.Magnitude)
	}
	if eq.Place != "Auckland Islands, New Zealand" {
		t.Errorf("Place = %q", eq.Place)
	}
	if eq.Lat != -51.2 {
		t.Errorf("Lat = %v, want -51.2", eq.Lat)
	}
	if eq.Lon != 174.5 {
		t.Errorf("Lon = %v, want 174.5", eq.Lon)
	}
	if eq.Depth != 10.0 {
		t.Errorf("Depth = %v, want 10.0", eq.Depth)
	}
	if eq.Time != 1718000000000 {
		t.Errorf("Time = %d, want 1718000000000", eq.Time)
	}
}

func TestListSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, featureCollectionJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.List(context.Background(), usgsquake.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent not sent")
	}
}

func TestNearbyUsesRadiusParam(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RawQuery
		_, _ = fmt.Fprint(w, featureCollectionJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Nearby(context.Background(), 35.0, 139.0, 200.0, 3.0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if gotURL == "" {
		t.Fatal("no query string")
	}
	// Check params were set
	for _, param := range []string{"latitude", "longitude", "maxradiuskm"} {
		if !containsParam(gotURL, param) {
			t.Errorf("missing param %q in query: %s", param, gotURL)
		}
	}
}

func TestCountParsesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, countJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	ct, err := c.Count(context.Background(), usgsquake.CountParams{MinMag: 4.0, Start: "2024-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	if ct.Count != 247 {
		t.Errorf("Count = %d, want 247", ct.Count)
	}
}

func TestGetByEventIDUsesParam(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = fmt.Fprint(w, singleFeatureJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	eq, err := c.Get(context.Background(), "us7000m6al")
	if err != nil {
		t.Fatal(err)
	}
	if eq.ID != "us7000m6al" {
		t.Errorf("ID = %q, want us7000m6al", eq.ID)
	}
	if !containsParam(gotQuery, "eventid") {
		t.Errorf("missing eventid param in query: %s", gotQuery)
	}
}

func TestListEmptyCollection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"type":"FeatureCollection","features":[]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	eqs, err := c.List(context.Background(), usgsquake.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(eqs) != 0 {
		t.Errorf("len(eqs) = %d, want 0", len(eqs))
	}
}

func TestRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, featureCollectionJSON)
	}))
	defer ts.Close()

	cfg := usgsquake.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := usgsquake.NewClient(cfg)

	_, err := c.List(context.Background(), usgsquake.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

// containsParam checks if a raw query string contains a specific key.
func containsParam(query, key string) bool {
	return len(query) > 0 && (query == key ||
		len(query) >= len(key)+1 && (query[:len(key)+1] == key+"=" ||
			contains(query, "&"+key+"=")))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
