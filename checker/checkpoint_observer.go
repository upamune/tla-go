package checker

// checkpointObserver records values only. In particular it accepts neither a
// callback nor an io.Writer: even Write can execute arbitrary application
// code. It is confined to the serial checkpoint search and its owner must drain
// records at safe boundaries. Records are output, not checkpoint state; the
// recovery driver must deliver them in order and handle its own output policy.
// The optional channel transports copies and never invokes caller callbacks.
type checkpointObserver struct {
	records []checkpointObservation
	stream  chan<- CheckpointObservation
}

type checkpointObservation struct {
	searchEvent *SearchEvent
	warning     *Warning
	print       *string
}

func (o *checkpointObserver) search(event SearchEvent) {
	o.add(checkpointObservation{searchEvent: &event})
}

func (o *checkpointObserver) warning(warning Warning) {
	o.add(checkpointObservation{warning: &warning})
}

func (o *checkpointObserver) print(line string) {
	o.add(checkpointObservation{print: &line})
}

func (o *checkpointObserver) add(record checkpointObservation) {
	o.records = append(o.records, record)
	o.send(record)
}

func (o *checkpointObserver) send(record checkpointObservation) {
	if o.stream != nil {
		// Send independent values: the receiver cannot mutate the persisted log.
		var copy CheckpointObservation
		if record.searchEvent != nil {
			v := *record.searchEvent
			copy.Search = &v
		}
		if record.warning != nil {
			v := *record.warning
			copy.Warning = &v
		}
		if record.print != nil {
			v := *record.print
			copy.Print = &v
		}
		o.stream <- copy
	}
}

func (o *checkpointObserver) startStream(stream chan<- CheckpointObservation) {
	o.stream = stream
	if stream != nil {
		for _, record := range o.records {
			o.send(record)
		}
	}
}
