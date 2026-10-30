package checker

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestUpstreamJSONFixtures(t *testing.T) {
	for _, name := range []string{"JsonTests", "JsonValues"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata", "json", name+".tla"))
			if err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile(filepath.Join("testdata", "json", name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			written := map[string]string{}
			options := Options{ReadFile: func(path string) (string, error) {
				if text, ok := written[path]; ok {
					return text, nil
				}
				data, err := os.ReadFile(filepath.Join("testdata", "json", path))
				if err != nil {
					return "", fs.ErrNotExist
				}
				return string(data), nil
			}, WriteFile: func(path, text string) error { written[path] = text; return nil }}
			got, err := CheckWithOptions(context.Background(), string(source), string(config), options)
			if err != nil || got.Status != Success {
				t.Fatalf("got %+v, %v", got, err)
			}
			if name == "JsonTests" && written["target/json/test.json"] != `{"a":3,"b":{"c":[4,5,6]}}` {
				t.Fatalf("written %q", written)
			}
			if name == "JsonValues" && len(written) != 0 {
				t.Fatalf("unexpected writes %q", written)
			}
		})
	}
}

func TestUpstreamJSONConversions(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Json, Integers, TLC, TLCExt
CONSTANT C
ASSUME AssertEq(ToJson(FALSE), "false")
ASSUME AssertEq(ToJson(1), "1")
ASSUME AssertEq(ToJson("a"), "\"a\"")
ASSUME AssertEq(ToJson(C), "\"ModelValue\"")
ASSUME AssertEq(ToJson({TRUE, FALSE}), "[false,true]")
ASSUME AssertEq(ToJson({3, 2, 1}), "[1,2,3]")
ASSUME AssertEq(ToJson(<<3, 2, 1>>), "[3,2,1]")
ASSUME AssertEq(ToJson([x \in {3, 2, 1} |-> 2 * x + 1]), "[3,5,7]")
ASSUME AssertEq(ToJson([a: {2, 1}, b: {"a", "b"}]), "[{\"a\":1,\"b\":\"a\"},{\"a\":1,\"b\":\"b\"},{\"a\":2,\"b\":\"a\"},{\"a\":2,\"b\":\"b\"}]")
ASSUME AssertEq(ToJsonObject([n \in {-1, 2} |-> n]), "{\"-1\":-1,\"2\":2}")
ASSUME AssertEq(ToJsonArray(SUBSET {2, 1}), "[[],[1],[2],[1,2]]")
====`
	got, err := Check(source, "CONSTANT C = ModelValue")
	if err != nil || got.Status != Success {
		t.Fatalf("got %+v, %v", got, err)
	}
}
