package infoblox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	ibclient "github.com/infobloxopen/infoblox-go-client/v2"
	"github.com/libdns/libdns"
)

// fakeRequestor implements ibclient.HttpRequestor with a routing function, so a
// test can answer WAPI calls with canned payloads and inspect what was sent.
// This exercises the real infoblox-go-client request-building and JSON decoding
// path — the part most likely to shift under a dependency bump.
type fakeRequestor struct {
	handle   func(req *http.Request, body []byte) (status int, resp []byte)
	requests []recordedRequest
}

type recordedRequest struct {
	method string
	path   string
	body   string
}

func (f *fakeRequestor) Init(ibclient.AuthConfig, ibclient.TransportConfig) {}

func (f *fakeRequestor) SendRequest(req *http.Request) ([]byte, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	f.requests = append(f.requests, recordedRequest{
		method: req.Method,
		path:   req.URL.Path,
		body:   string(body),
	})

	status, resp := f.handle(req, body)
	if status >= 400 {
		return nil, errors.New("wapi error status")
	}
	return resp, nil
}

func newTestProvider(f *fakeRequestor) *Provider {
	return &Provider{
		Host:      "infoblox.test",
		Version:   "2.12",
		Username:  "admin",
		Password:  "infoblox",
		requestor: f,
	}
}

func TestView(t *testing.T) {
	tests := []struct {
		name string
		view string
		want string
	}{
		{"empty falls back to default", "", "default"},
		{"explicit value is kept", "internal", "internal"},
		{"default stays default", "default", "default"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Provider{View: tc.view}
			if got := p.view(); got != tc.want {
				t.Fatalf("view() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetRecords(t *testing.T) {
	f := &fakeRequestor{
		handle: func(req *http.Request, _ []byte) (int, []byte) {
			switch {
			case strings.Contains(req.URL.Path, "record:cname"):
				return 200, []byte(`[
					{"_ref":"record:cname/a","name":"www.example.com","canonical":"lb.example.com","view":"default"},
					{"_ref":"record:cname/b","name":"api.example.com","canonical":"lb.example.com","view":"default"}
				]`)
			case strings.Contains(req.URL.Path, "record:txt"):
				return 200, []byte(`[
					{"_ref":"record:txt/c","name":"_acme-challenge.example.com","text":"token-123","view":"default"}
				]`)
			default:
				return 200, []byte(`[]`)
			}
		},
	}

	got, err := newTestProvider(f).GetRecords(context.Background(), "example.com.")
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3: %+v", len(got), got)
	}

	// The zone suffix must be stripped from returned names.
	want := map[string]string{ // name -> data
		"www":             "lb.example.com",
		"api":             "lb.example.com",
		"_acme-challenge": "token-123",
	}
	for _, rec := range got {
		rr := rec.RR()
		data, ok := want[rr.Name]
		if !ok {
			t.Errorf("unexpected record name %q", rr.Name)
			continue
		}
		if rr.Data != data {
			t.Errorf("record %q: data = %q, want %q", rr.Name, rr.Data, data)
		}
		delete(want, rr.Name)
	}
	for name := range want {
		t.Errorf("missing expected record %q", name)
	}
}

func TestAppendRecordsSendsConfiguredView(t *testing.T) {
	var createBody string
	f := &fakeRequestor{
		handle: func(req *http.Request, body []byte) (int, []byte) {
			switch {
			// Create: POST record:cname -> returns the new object's ref as a JSON string.
			case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "record:cname"):
				createBody = string(body)
				return 201, []byte(`"record:cname/new"`)
			// Read-back by ref after create.
			case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "record:cname/new"):
				return 200, []byte(`{"_ref":"record:cname/new","name":"www.example.com","canonical":"lb.example.com","view":"internal"}`)
			default:
				return 200, []byte(`[]`)
			}
		},
	}

	p := newTestProvider(f)
	p.View = "internal"

	added, err := p.AppendRecords(context.Background(), "example.com.", []libdns.Record{
		libdns.RR{Type: "CNAME", Name: "www", Data: "lb.example.com"},
	})
	if err != nil {
		t.Fatalf("AppendRecords: %v", err)
	}
	if len(added) != 1 {
		t.Fatalf("got %d added, want 1", len(added))
	}
	if got := added[0].RR().Name; got != "www" {
		t.Errorf("added name = %q, want %q", got, "www")
	}
	if !strings.Contains(createBody, `"view":"internal"`) {
		t.Errorf("create payload missing configured view; body = %s", createBody)
	}
}

func TestAppendRecordsDefaultsViewOnWire(t *testing.T) {
	var createBody string
	f := &fakeRequestor{
		handle: func(req *http.Request, body []byte) (int, []byte) {
			switch {
			case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "record:txt"):
				createBody = string(body)
				return 201, []byte(`"record:txt/new"`)
			case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "record:txt/new"):
				return 200, []byte(`{"_ref":"record:txt/new","name":"_acme-challenge.example.com","text":"tok","view":"default"}`)
			default:
				return 200, []byte(`[]`)
			}
		},
	}

	// No View set -> the "default" fallback must reach the wire.
	_, err := newTestProvider(f).AppendRecords(context.Background(), "example.com.", []libdns.Record{
		libdns.RR{Type: "TXT", Name: "_acme-challenge", Data: "tok"},
	})
	if err != nil {
		t.Fatalf("AppendRecords: %v", err)
	}
	if !strings.Contains(createBody, `"view":"default"`) {
		t.Errorf("create payload should default view to \"default\"; body = %s", createBody)
	}
}
