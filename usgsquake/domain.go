// Package usgsquake exposes the USGS Earthquake catalog as a kit Domain driver.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/usgsquake-cli/usgsquake"
//
// The same Domain also builds the standalone quake binary (see cli.NewApp).
package usgsquake

import (
	"context"
	"fmt"
	"time"

	"github.com/tamnd/any-cli/kit"
)

func init() { kit.Register(Domain{}) }

// Domain is the usgsquake driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "usgsquake",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "quake",
			Short:  "USGS earthquake catalog",
			Long: `quake queries the USGS FDSNWS earthquake catalog for seismic events worldwide.

No API key required. Data from the USGS Comprehensive Earthquake Catalog (ComCat).`,
			Site: Host,
			Repo: "https://github.com/tamnd/usgsquake-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "list",
		Group:   "read",
		List:    true,
		Summary: "List earthquakes by magnitude and date range",
	}, listOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "nearby",
		Group:   "read",
		List:    true,
		Summary: "Find earthquakes near a geographic point",
		Args: []kit.Arg{
			{Name: "lat", Help: "latitude in decimal degrees"},
			{Name: "lon", Help: "longitude in decimal degrees"},
		},
	}, nearbyOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "count",
		Group:   "read",
		Single:  true,
		Summary: "Count earthquakes matching criteria",
	}, countOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "get",
		Group:   "read",
		Single:  true,
		Summary: "Fetch a single earthquake event by USGS ID",
		Args:    []kit.Arg{{Name: "event_id", Help: "USGS event ID (e.g. us7000m6al)"}},
	}, getOp)
}

// newClient builds the USGS client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- input structs ---

type listInput struct {
	MinMag float64 `kit:"flag" help:"minimum magnitude" default:"4.0"`
	MaxMag float64 `kit:"flag" help:"maximum magnitude"`
	Start  string  `kit:"flag" help:"start date (YYYY-MM-DD)"`
	End    string  `kit:"flag" help:"end date (YYYY-MM-DD)"`
	Limit  int     `kit:"flag,inherit" help:"max results" default:"20"`
	Client *Client `kit:"inject"`
}

type nearbyInput struct {
	Lat    string  `kit:"arg" help:"latitude in decimal degrees"`
	Lon    string  `kit:"arg" help:"longitude in decimal degrees"`
	Radius float64 `kit:"flag" help:"search radius in km" default:"200"`
	MinMag float64 `kit:"flag" help:"minimum magnitude" default:"3.0"`
	Limit  int     `kit:"flag,inherit" help:"max results" default:"10"`
	Client *Client `kit:"inject"`
}

type countInput struct {
	MinMag float64 `kit:"flag" help:"minimum magnitude" default:"4.0"`
	Start  string  `kit:"flag" help:"start date (YYYY-MM-DD)"`
	End    string  `kit:"flag" help:"end date (YYYY-MM-DD)"`
	Client *Client `kit:"inject"`
}

type getInput struct {
	EventID string  `kit:"arg" help:"USGS event ID (e.g. us7000m6al)"`
	Client  *Client `kit:"inject"`
}

// --- handlers ---

func listOp(ctx context.Context, in listInput, emit func(Earthquake) error) error {
	start := in.Start
	if start == "" {
		start = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	eqs, err := in.Client.List(ctx, ListParams{
		MinMag: in.MinMag,
		MaxMag: in.MaxMag,
		Start:  start,
		End:    in.End,
		Limit:  in.Limit,
	})
	if err != nil {
		return err
	}
	for _, eq := range eqs {
		if err := emit(eq); err != nil {
			return err
		}
	}
	return nil
}

func nearbyOp(ctx context.Context, in nearbyInput, emit func(Earthquake) error) error {
	var lat, lon float64
	if _, err := fmt.Sscanf(in.Lat, "%f", &lat); err != nil {
		return fmt.Errorf("invalid lat %q: %w", in.Lat, err)
	}
	if _, err := fmt.Sscanf(in.Lon, "%f", &lon); err != nil {
		return fmt.Errorf("invalid lon %q: %w", in.Lon, err)
	}
	eqs, err := in.Client.Nearby(ctx, lat, lon, in.Radius, in.MinMag, in.Limit)
	if err != nil {
		return err
	}
	for _, eq := range eqs {
		if err := emit(eq); err != nil {
			return err
		}
	}
	return nil
}

func countOp(ctx context.Context, in countInput, emit func(*Count) error) error {
	start := in.Start
	if start == "" {
		start = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	ct, err := in.Client.Count(ctx, CountParams{
		MinMag: in.MinMag,
		Start:  start,
		End:    in.End,
	})
	if err != nil {
		return err
	}
	return emit(ct)
}

func getOp(ctx context.Context, in getInput, emit func(*Earthquake) error) error {
	eq, err := in.Client.Get(ctx, in.EventID)
	if err != nil {
		return err
	}
	return emit(eq)
}

// Classify turns any input into the canonical (type, id).
func (Domain) Classify(input string) (string, string, error) {
	return "event", input, nil
}

// Locate returns the live USGS URL for a (type, id).
func (Domain) Locate(t, id string) (string, error) {
	return "https://earthquake.usgs.gov/earthquakes/eventpage/" + id, nil
}
