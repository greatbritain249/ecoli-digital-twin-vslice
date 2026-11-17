package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPISessions(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "data"), 0755)
	_ = os.WriteFile(filepath.Join(root, "data", "catalog.json"), []byte(`{"structures":[]}`), 0644)
	srv := New(root)
	defer srv.Close()
	handler := srv.Handler()
	req := httptest.NewRequest("POST", "/api/simulations", strings.NewReader(`{"medium":"rich"}`))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != 201 {
		t.Fatalf("create %d: %s", res.Code, res.Body.String())
	}
	var x struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &x); err != nil || len(x.ID) != 32 {
		t.Fatal("invalid ID")
	}
	for _, action := range []string{`{"action":"start"}`, `{"action":"set_medium","medium":"starved"}`, `{"action":"pause"}`} {
		res = httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest("POST", "/api/simulations/"+x.ID+"/actions", strings.NewReader(action)))
		if res.Code != 200 {
			t.Fatalf("action %s: %s", action, res.Body.String())
		}
	}
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest("GET", "/api/catalog", nil))
	if res.Code != 200 {
		t.Fatalf("catalog failed: %d", res.Code)
	}
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest("DELETE", "/api/simulations/"+x.ID, nil))
	if res.Code != 204 {
		t.Fatalf("delete %d", res.Code)
	}
}
func TestReplayAPI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0755); err != nil {
		t.Fatal(err)
	}
	trace := `{"schema_version":1,"source":"synthetic test","frames":[{"time_min":0,"medium":"rich","glucose_mM":3,"atp_mM":2.4,"biomass_fg":280,"translation_activity":0.65},{"time_min":2,"medium":"rich","glucose_mM":2.9,"atp_mM":2.3,"biomass_fg":283,"translation_activity":0.64}]}`
	if err := os.WriteFile(filepath.Join(root, "data", "replay-example.json"), []byte(trace), 0644); err != nil {
		t.Fatal(err)
	}
	srv := New(root)
	defer srv.Close()
	res := httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, httptest.NewRequest("GET", "/api/replay/info", nil))
	if res.Code != 200 || !strings.Contains(res.Body.String(), "synthetic test") {
		t.Fatalf("replay info: %v", res.Body.String())
	}
	res = httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, httptest.NewRequest("POST", "/api/simulations", strings.NewReader(`{"mode":"replay"}`)))
	if res.Code != 201 || !strings.Contains(res.Body.String(), "imported-trace") {
		t.Fatalf("replay create: %v", res.Body.String())
	}
}
