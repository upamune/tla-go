package checker

import (
	"context"
	"reflect"
	"testing"
)

func TestViewFailureHeaderPrecedesReplay(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nV == IF PrintT(x) THEN UNION {1} ELSE {}\n===="
	var events []string
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next VIEW V CHECK_DEADLOCK FALSE", Options{
		OnPrint: func(text string) { events = append(events, "print "+text) },
		OnFailureHeader: func(failure Failure) {
			if failure.ToolCode != 2102 || failure.InitialState == nil {
				t.Errorf("header %+v", failure)
			}
			events = append(events, "header")
		},
	})
	if err != nil || result.Failure == nil || !result.Failure.HeaderReported || !reflect.DeepEqual(events, []string{"print 0", "header", "print 0"}) {
		t.Fatalf("result %+v, error %v, events %q", result, err, events)
	}
}
