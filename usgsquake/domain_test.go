package usgsquake

import (
	"testing"
)

// These tests are offline: they exercise the domain's metadata and pure string
// functions which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "usgsquake" {
		t.Errorf("Scheme = %q, want usgsquake", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "quake" {
		t.Errorf("Identity.Binary = %q, want quake", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	typ, id, err := Domain{}.Classify("us7000m6al")
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if typ != "event" {
		t.Errorf("type = %q, want event", typ)
	}
	if id != "us7000m6al" {
		t.Errorf("id = %q, want us7000m6al", id)
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("event", "us7000m6al")
	if err != nil {
		t.Fatalf("Locate error: %v", err)
	}
	want := "https://earthquake.usgs.gov/earthquakes/eventpage/us7000m6al"
	if got != want {
		t.Errorf("Locate = %q, want %q", got, want)
	}
}
