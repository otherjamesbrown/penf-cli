package buildinfo

import (
	"encoding/json"
	"net/http"
	"runtime"
)

// These vars are set at build time via ldflags:
// -X github.com/otherjamesbrown/penf-cli/pkg/buildinfo.Version=v0.8.2
// -X github.com/otherjamesbrown/penf-cli/pkg/buildinfo.Commit=b806fe7
// -X github.com/otherjamesbrown/penf-cli/pkg/buildinfo.BuildTime=2026-02-07T10:30:00Z
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Info holds build information for a service.
type Info struct {
	ServiceName string `json:"service_name"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	BuildTime   string `json:"build_time"`
	GoVersion   string `json:"go_version"`
}

// Get returns build info for the named service.
func Get(serviceName string) Info {
	return Info{
		ServiceName: serviceName,
		Version:     Version,
		Commit:      Commit,
		BuildTime:   BuildTime,
		GoVersion:   runtime.Version(),
	}
}

// String returns a human-readable one-liner like "v0.8.2 (b806fe7, 2026-02-07T10:30:00Z)"
func String() string {
	return Version + " (" + Commit + ", " + BuildTime + ")"
}

// Handler returns an HTTP handler that responds with build info JSON.
func Handler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		info := Get(serviceName)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}
}
