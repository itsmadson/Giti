package filter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type corpusCase struct {
	CQL  string `json:"cql"`
	SQL  string `json:"sql"`
	Args []any  `json:"args"`
}

func TestGoldenCorpus(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "tests", "filter-corpus", "corpus.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []corpusCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 30 {
		t.Fatalf("corpus too small: %d", len(cases))
	}
	for _, c := range cases {
		e, err := ParseCQL(c.CQL)
		if err != nil {
			t.Errorf("%s: parse: %v", c.CQL, err)
			continue
		}
		sql, args, err := ToSQL(e, 1)
		if err != nil {
			t.Errorf("%s: tosql: %v", c.CQL, err)
			continue
		}
		if sql != c.SQL {
			t.Errorf("%s:\n got  %s\n want %s", c.CQL, sql, c.SQL)
		}
		if len(args) != len(c.Args) {
			t.Errorf("%s: args %v want %v", c.CQL, args, c.Args)
			continue
		}
		for i := range args {
			if !jsonArgEqual(args[i], c.Args[i]) {
				t.Errorf("%s: arg[%d] %#v want %#v", c.CQL, i, args[i], c.Args[i])
			}
		}
	}
}

// jsonArgEqual compares a Go arg (string/float64/bool) against a JSON-decoded
// expected value (json numbers are float64).
func jsonArgEqual(got, want any) bool {
	return got == want
}
