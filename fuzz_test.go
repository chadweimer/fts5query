package fts5query_test

import (
	"database/sql"
	"testing"

	"github.com/chadweimer/fts5query"
	_ "modernc.org/sqlite"
)

func FuzzParse(f *testing.F) {
	sqliteDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		f.Fatalf("failed to open sqlite db: %v", err)
	}
	defer func() { _ = sqliteDB.Close() }()

	_, err = sqliteDB.Exec(`CREATE VIRTUAL TABLE fuzz_fts USING fts5(
		title,
		body,
		tokenize='porter unicode61'
	)`)
	if err != nil {
		f.Fatalf("failed to create fuzz_fts table: %v", err)
	}

	// Seed corpus with realistic, tricky, and adversarial inputs
	seeds := []string{
		"apple banana",
		`"chocolate chip"`,
		`"unclosed phrase`,
		"-walnuts -peanuts",
		"apple OR banana -nuts",
		"apple OR -nuts banana",
		"apple -nuts OR banana",
		"OR OR OR",
		": * ^ { } ( ) ~ + & | -",
		"3/4 cup! 1.5 grandma's all-purpose",
		"mac AND cheese NOT ham",
		`"""`,
		`"foo"bar"baz"`,
		"--- ---",
		"こんにちは 世界",
		"cafe\u0301 re\u0301sume\u0301",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		res := fts5query.Parse(input, fts5query.WithColumns("title", "body"))
		if res == nil {
			t.Fatal("Parse returned nil result")
		}

		if !res.IsEmpty() {
			rows, err := sqliteDB.Query("SELECT rowid FROM fuzz_fts WHERE fuzz_fts MATCH ?", res.MatchExpr)
			if err != nil {
				t.Fatalf("generated invalid FTS5 syntax for input %q -> MatchExpr %q: %v", input, res.MatchExpr, err)
			}
			_ = rows.Close()
		}
	})
}
