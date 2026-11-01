package upstreamjunit

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

const tinySweep = `    integration_test.go:84: unresolved example.A spec="A"
    integration_test.go:112: diagnostic example.A differences=[message:2192 exit] selected bodies=[2199 2192] presence=[2186 1000]; partial comparison, liveness strategy unresolved, not a match; Java exit=0 Go exit=1
    integration_test.go:84: unresolved example.B spec="B"
    integration_test.go:102: diagnostic example.B comparison unavailable: java timed out: context deadline exceeded
    integration_test.go:84: unresolved example.C spec="C"
    integration_test.go:123: incomplete sweep: 3 candidates with recovered spec, 2 diagnostic runs, 1 candidates with unsafe or unresolved setup, 3 unresolved non-abstract candidates; inventory is not an exhaustive JUnit count
--- FAIL: TestUpstreamJUnitSweep (1120.44s)
`

func TestParseSweepLog(t *testing.T) {
	got, err := ParseSweepLog(strings.NewReader(tinySweep))
	if err != nil {
		t.Fatal(err)
	}
	want := SweepCounts{Candidates: 3, Attempted: 2, Unsafe: 1, Compared: 1, Differences: 1, Unavailable: 1}
	if got.Counts != want {
		t.Fatalf("counts: %+v", got.Counts)
	}
	if !reflect.DeepEqual(got.Classes[0].Differences, []string{"exit", "message:2192"}) || *got.Classes[0].GoExit != 1 || got.Classes[1].Unavailable != "java_timeout" || got.Classes[2].Status != "unsafe_setup" {
		t.Fatalf("records: %+v", got.Classes)
	}
	again, err := ParseSweepLog(strings.NewReader(tinySweep))
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("nondeterministic: %v", err)
	}
}

func TestParseSweepLogRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"duplicate candidate":  strings.Replace(tinySweep, "    integration_test.go:84: unresolved example.B", "    integration_test.go:84: unresolved example.A spec=\"A\"\n    integration_test.go:84: unresolved example.B", 1),
		"duplicate diagnostic": strings.Replace(tinySweep, "    integration_test.go:84: unresolved example.B", "    integration_test.go:102: diagnostic example.A comparison unavailable: java timed out: context deadline exceeded\n    integration_test.go:84: unresolved example.B", 1),
		"bad counts":           strings.Replace(tinySweep, "2 diagnostic runs", "1 diagnostic runs", 1),
		"missing result":       strings.Replace(tinySweep, "--- FAIL: TestUpstreamJUnitSweep (1120.44s)", "", 1),
		"bad difference":       strings.Replace(tinySweep, "message:2192 exit", "message:oops exit", 1),
		"bad diagnostic":       strings.Replace(tinySweep, "Java exit=0 Go exit=1", "Java exit=x Go exit=1", 1),
		"unknown unavailable":  strings.Replace(tinySweep, "java timed out: context deadline exceeded", "other error", 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSweepLog(strings.NewReader(input)); err == nil {
				t.Fatal("accepted invalid log")
			}
		})
	}
}

func TestSweepSnapshotConsistency(t *testing.T) {
	for _, tc := range []struct {
		file, commit, hash string
		counts             SweepCounts
	}{
		{"sweep-2399b153.json", "2399b1535d1ef68aa78d81bba2b9559cf8e586f7", "4769aef155298934c93845b6a0130091bbd0f1060d03ebd94762068acdcb755a", SweepCounts{340, 309, 31, 276, 224, 52, 33}},
		{"sweep-56336d54.json", "56336d54f659dbb744aff66ffa8e653a6117c06a", "fa1bf74a467a45cbd1d029017dbafb7ae0305221d5f1e5e91201fc7a6cc29ec5", SweepCounts{340, 309, 31, 293, 236, 57, 16}},
		{"sweep-e81f221f.json", "e81f221f7ba59b833244e7613bd149f836532c86", "65e957a50c1a58779f5982458ca42adb3d4ed16e0884229dd7bbe752aa298765", SweepCounts{340, 309, 31, 294, 254, 40, 15}},
		{"sweep-9f581626.json", "9f5816269083eb7c9d44e5477695e95e6bf68059", "e548e0fba387f59c1278069d2bb0bcd7196475e36a6049003336d2616bb13000", SweepCounts{340, 309, 31, 295, 256, 39, 14}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile("testdata/snapshots/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			var snap SweepSnapshot
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&snap); err != nil {
				t.Fatal(err)
			}
			if snap.Schema != 1 || snap.TlaGoCommit != tc.commit || snap.UpstreamCommit != UpstreamCommit || snap.JarSHA256 != JarSHA256 || snap.SourceSHA256 != tc.hash {
				t.Fatal("incorrect provenance")
			}
			counts := SweepCounts{Candidates: len(snap.Classes)}
			last := ""
			for _, r := range snap.Classes {
				if r.Class <= last {
					t.Fatalf("duplicate or unsorted class: %s", r.Class)
				}
				last = r.Class
				switch r.Status {
				case "unsafe_setup":
					counts.Unsafe++
					if r.JavaExit != nil || r.GoExit != nil || r.Unavailable != "" {
						t.Fatal(r)
					}
				case "unavailable":
					counts.Attempted++
					counts.Unavailable++
					if r.JavaExit != nil || r.GoExit != nil || r.Unavailable == "" {
						t.Fatal(r)
					}
				case "compared":
					counts.Attempted++
					counts.Compared++
					if r.JavaExit == nil || r.GoExit == nil || r.Unavailable != "" {
						t.Fatal(r)
					}
					if len(r.Differences) == 0 {
						counts.NoDifferences++
					} else {
						counts.Differences++
					}
				default:
					t.Fatalf("invalid status: %s", r.Status)
				}
			}
			if counts != tc.counts || snap.Counts != counts {
				t.Fatalf("counts: %+v / %+v", counts, snap.Counts)
			}
		})
	}
}
