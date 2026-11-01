package checker

// checkpointObserver records values only. In particular it accepts neither a
// callback nor an io.Writer: even Write can execute arbitrary application
// code. It is confined to the serial checkpoint search and its owner must drain
// records at safe boundaries. Records are output, not checkpoint state; the
// recovery driver must deliver them in order and handle its own output policy.
// This is deliberately private until a recovery driver can provide that seam
// without granting arbitrary API callbacks checkpoint trust.
type checkpointObserver struct {
	records []checkpointObservation
}

type checkpointObservation struct {
	searchEvent *SearchEvent
	warning     *Warning
	print       *string
}

func (o *checkpointObserver) search(event SearchEvent) {
	o.records = append(o.records, checkpointObservation{searchEvent: &event})
}

func (o *checkpointObserver) warning(warning Warning) {
	o.records = append(o.records, checkpointObservation{warning: &warning})
}

func (o *checkpointObserver) print(line string) {
	o.records = append(o.records, checkpointObservation{print: &line})
}
