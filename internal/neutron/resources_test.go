package neutron

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestObserve drives Observe over an httptest cloud, covering a live resource, a
// 404 mapped to gone (the idempotency edge), a server error propagated, and a
// statusless kind that still reports as existing.
func TestObserve(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/networks/live":
			_, _ = io.WriteString(w, `{"network":{"id":"live","status":"ACTIVE"}}`)
		case "/networks/gone":
			w.WriteHeader(http.StatusNotFound)
		case "/networks/broken":
			w.WriteHeader(http.StatusInternalServerError)
		case "/subnets/sub-1":
			_, _ = io.WriteString(w, `{"subnet":{"id":"sub-1"}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	c := testServiceClient(ts)

	cases := []struct {
		name       string
		res        Resource
		wantStatus string
		wantExists bool
		wantErr    bool
	}{
		{"live network", Resource{Kind: KindNetwork, ID: "live"}, "ACTIVE", true, false},
		{"deleted resource reads as gone", Resource{Kind: KindNetwork, ID: "gone"}, "", false, false},
		{"server error propagates", Resource{Kind: KindNetwork, ID: "broken"}, "", false, true},
		{"statusless kind exists", Resource{Kind: KindSubnet, ID: "sub-1"}, "", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, exists, err := c.Observe(context.Background(), tc.res)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Observe(%v): expected an error, got nil", tc.res)
				}
				return
			}
			if err != nil {
				t.Fatalf("Observe(%v): unexpected error %v", tc.res, err)
			}
			if status != tc.wantStatus {
				t.Errorf("status = %q, want %q", status, tc.wantStatus)
			}
			if exists != tc.wantExists {
				t.Errorf("exists = %v, want %v", exists, tc.wantExists)
			}
		})
	}
}
