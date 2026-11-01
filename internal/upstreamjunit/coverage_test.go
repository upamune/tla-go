package upstreamjunit

import (
	"reflect"
	"testing"
)

func TestZeroCoverageRecorderProjection(t *testing.T) {
	output := "@!@!@STARTMSG 2201:0 @!@!@\nstart\n@!@!@ENDMSG 2201 @!@!@\n" +
		"@!@!@STARTMSG 2221:0 @!@!@\n  |line 1, col 2 to line 1, col 3 of module M: 0\n@!@!@ENDMSG 2221 @!@!@\n" +
		"@!@!@STARTMSG 2775:0 @!@!@\n  line 2: 0:0\n@!@!@ENDMSG 2775 @!@!@\n" +
		"@!@!@STARTMSG 2221:0 @!@!@\n  line 3: 2\n@!@!@ENDMSG 2221 @!@!@\n" +
		"@!@!@STARTMSG 2202:0 @!@!@\nend\n@!@!@ENDMSG 2202 @!@!@\n"
	got, err := ZeroCoverage(Result{Output: output})
	if err != nil || !reflect.DeepEqual(got, []string{"line 1, col 2 to line 1, col 3 of module M"}) {
		t.Fatalf("zero coverage = %v, %v", got, err)
	}
	if _, err := ZeroCoverage(Result{Output: "@!@!@STARTMSG 2201:0 @!@!@\nstart\n@!@!@ENDMSG 2201 @!@!@\n"}); err == nil {
		t.Fatal("incomplete report accepted")
	}
}
