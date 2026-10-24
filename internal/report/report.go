package report

import (
	"encoding/json"
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"io"
	"strings"
	"unicode"
)

func Write(w io.Writer, r types.Report, format string) error {
	if format == "json" {
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		return e.Encode(r)
	}
	if format != "text" {
		return fmt.Errorf("unsupported format %q", format)
	}
	out := w
	var buffer strings.Builder
	w = &buffer
	if _, e := fmt.Fprintf(w, "SystemSync Diagnostic Report\nCollected: %s  Platform: %s\n", r.Timestamp.Format("2006-01-02T15:04:05Z07:00"), r.Platform); e != nil {
		return e
	}
	for _, o := range r.Observations {
		fmt.Fprintf(w, "\n%s (%s)\n", o.Source, o.Timestamp.Format("15:04:05Z07:00"))
		if o.Error != "" {
			fmt.Fprintf(w, "  Unavailable/partial: %s\n", o.Error)
		}
		if o.Data != nil {
			b, e := json.MarshalIndent(o.Data, "  ", "  ")
			if e != nil {
				return e
			}
			fmt.Fprintf(w, "  %s\n", b)
		}
	}
	fmt.Fprintln(w, "\nFindings:")
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "  No configured rule triggered. Unavailable observations are not a clean bill of health.")
	}
	for _, f := range r.Findings {
		fmt.Fprintf(w, "  [%s] %s (%s) [%s]\n    %s\n", f.Severity, f.Title, f.Confidence, f.ID, f.Explanation)
		for _, e := range f.Evidence {
			fmt.Fprintf(w, "    Evidence: %s\n", e)
		}
		for _, s := range f.Steps {
			fmt.Fprintf(w, "    Next: %s\n", s)
		}
	}
	if r.Explanation != "" {
		fmt.Fprintf(w, "\nExplanation:\n%s\n", r.Explanation)
	}
	text := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, buffer.String())
	_, err := io.WriteString(out, text)
	return err
}
