package report

import (
	"bytes"
	"encoding/json"
	"github.com/manitrix/systemsync/pkg/types"
	"testing"
)

func TestJSON(t *testing.T) {
	var b bytes.Buffer
	r := types.Report{SchemaVersion: "1", Observations: []types.Observation{{Source: "missing", Error: "permission denied"}}, Findings: []types.Finding{}}
	if e := Write(&b, r, "json"); e != nil {
		t.Fatal(e)
	}
	var decoded types.Report
	if e := json.Unmarshal(b.Bytes(), &decoded); e != nil || decoded.SchemaVersion != "1" || decoded.Observations[0].Error == "" {
		t.Fatalf("%s %v", b.String(), e)
	}
	if e := Write(&b, r, "invalid"); e == nil {
		t.Fatal("invalid format")
	}
}
