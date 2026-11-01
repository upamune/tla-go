package main

import (
	"fmt"
	"io"
	"time"

	"github.com/upamune/tla-go/checker"
)

func temporalProgressStats(event checker.SearchEvent) string {
	return fmt.Sprintf("Progress(%d) at %s: %d states generated, %d distinct states found, 0 states left on queue.", event.Level, toolTime(), event.GeneratedStates, event.DistinctStates)
}

func printTemporalCheckEnd(out io.Writer, tool bool, elapsed time.Duration) {
	seconds := int64(elapsed / time.Second)
	duration := fmt.Sprintf("%02ds", seconds%60)
	if elapsed.Milliseconds() > 86400000 {
		date := time.UnixMilli(elapsed.Milliseconds() - 86400000).UTC()
		duration = fmt.Sprintf("%dd %02dh", date.YearDay(), date.Hour())
	} else if elapsed.Milliseconds() > 3600000 {
		duration = fmt.Sprintf("%02dh %02dmin", seconds/3600%24, seconds/60%60)
	} else if elapsed.Milliseconds() > 60000 {
		duration = fmt.Sprintf("%02dmin %02ds", seconds/60%60, seconds%60)
	}
	body := fmt.Sprintf("Finished checking temporal properties in %s at %s", duration, toolTime())
	if tool {
		(toolOutput{out: out}).message(2267, 0, body)
	} else {
		fmt.Fprintln(out, body)
	}
}
