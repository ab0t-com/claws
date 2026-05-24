package hints

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// RenderText writes a "Next:" block to w, formatted for humans. Produces
// nothing if the set is empty or the profile is off.
//
// Format:
//
//	Next:
//	  process pending: 4 agents never started   claws start-all
//	  inspect the healthy one                   claws agent ping team1/ben
//
// Two-space indent. Columns aligned via text/tabwriter. The Reason column
// is shown only when present.
//
// A leading blank line is written before the "Next:" header — callers
// should NOT add their own separator.
func RenderText(w io.Writer, set HintSet) {
	if w == nil || set.Profile == ProfileOff || len(set.Hints) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next:")
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, h := range set.Hints {
		if h.Reason != "" {
			fmt.Fprintf(tw, "  %s\t%s\n", h.Reason, h.Command)
		} else {
			fmt.Fprintf(tw, "  %s\t%s\n", h.Name, h.Command)
		}
	}
	_ = tw.Flush()
}
