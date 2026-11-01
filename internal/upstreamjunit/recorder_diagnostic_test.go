package upstreamjunit

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

func TestDecodePartialRecorderPreservesTypedCalls(t *testing.T) {
	stream := `{"kind":"header","version":1,"complete":false}` + "\n" +
		`{"kind":"record","complete":false,"code":2110,"array":"strings","values":["Inv"]}` + "\n" +
		`{"kind":"record","complete":false,"code":2121,"array":"strings","values":[]}` + "\n" +
		`{"kind":"record","complete":false,"code":2110,"array":"strings","values":["Other"]}` + "\n" +
		fmt.Sprintf(`{"kind":"omission","complete":false,"code":2193,"reason":%q}`+"\n", recorderdebug.SuccessProbabilityOmission)
	got, omissions, err := decodePartialRecorder(strings.NewReader(stream))
	want := []recorderRecord{
		{code: "2110", array: "strings", values: []recorderValue{{kind: "s", text: "Inv"}}},
		{code: "2121", array: "strings"},
		{code: "2110", array: "strings", values: []recorderValue{{kind: "s", text: "Other"}}},
	}
	if err != nil || !reflect.DeepEqual(got, want) || len(omissions) != 2 || bridgeVerdict(nil, omissions) == nil {
		t.Fatalf("records=%+v omissions=%v err=%v", got, omissions, err)
	}
	if _, err := encodeRecorder(got); err != nil {
		t.Fatalf("typed transfer rejected observed calls: %v", err)
	}
}

func TestDecodePartialRecorderRejectsUnknownOrIncompleteInput(t *testing.T) {
	header := `{"kind":"header","version":1,"complete":false}` + "\n"
	for _, input := range []string{
		"",
		header + `{"kind":"record","complete":false,"code":9999,"array":"strings","values":[]}` + "\n",
		header + `{"kind":"record","complete":false,"code":2110,"array":"objects","values":["Inv"]}` + "\n",
		header + `{"kind":"record","complete":false,"code":2110,"array":"strings","values":["Inv"]}`,
	} {
		if records, omissions, err := decodePartialRecorder(strings.NewReader(input)); err == nil || records != nil || omissions != nil {
			t.Fatalf("accepted incomplete input: records=%v omissions=%v error=%v", records, omissions, err)
		}
	}
}
