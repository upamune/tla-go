package parser

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

type corpusSample struct {
	Name, Source string
	Want         Module
}

func TestUpstreamCorpusProjection(t *testing.T) {
	f, err := os.Open("testdata/upstream-corpus.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	decoder := json.NewDecoder(z)
	count := 0
	for {
		var sample corpusSample
		err := decoder.Decode(&sample)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			got, err := ParseModule(sample.Source)
			if err != nil {
				t.Fatal(err)
			}
			got = upstreamProjection(t, got)
			if !reflect.DeepEqual(*got, sample.Want) {
				t.Fatalf("parsed AST differs from the upstream projection")
			}
		})
	}
	if count != 132 {
		t.Fatalf("expected 132 upstream AST samples, got %d", count)
	}
}
