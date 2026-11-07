package fts5query_test

import (
	"database/sql"
	"testing"

	"github.com/chadweimer/fts5query"
	_ "modernc.org/sqlite"
)

func TestParse(t *testing.T) {
	sqliteDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	defer func() { _ = sqliteDB.Close() }()

	_, err = sqliteDB.Exec(`CREATE VIRTUAL TABLE test_fts USING fts5(
		name,
		ingredients,
		directions,
		tokenize='porter unicode61'
	)`)
	if err != nil {
		t.Fatalf("failed to create test_fts table: %v", err)
	}

	tests := []struct {
		name              string
		rawQuery          string
		opts              []fts5query.Option
		expectedMatchExpr string
		expectedIsNeg     bool
		expectedIsEmpty   bool
	}{
		{
			name:            "empty query",
			rawQuery:        "",
			expectedIsEmpty: true,
		},
		{
			name:            "whitespace query",
			rawQuery:        "    \t\n  ",
			expectedIsEmpty: true,
		},
		{
			name:              "single word without columns",
			rawQuery:          "cookie",
			expectedMatchExpr: `("cookie"*)`,
		},
		{
			name:              "single word with columns",
			rawQuery:          "cookie",
			opts:              []fts5query.Option{fts5query.WithColumns("name", "ingredients")},
			expectedMatchExpr: `{name ingredients} : ("cookie"*)`,
		},
		{
			name:              "multiple words",
			rawQuery:          "chocolate chip",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("chocolate"* "chip"*)`,
		},
		{
			name:              "exact quoted phrase",
			rawQuery:          `"chocolate chip"`,
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("chocolate chip")`,
		},
		{
			name:              "unclosed quote treated as phrase",
			rawQuery:          `"chocolate chip`,
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("chocolate chip")`,
		},
		{
			name:              "positive term with negation",
			rawQuery:          "chocolate -nuts",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : (("chocolate"*) NOT "nuts"*)`,
		},
		{
			name:              "pure negation",
			rawQuery:          "-nuts -dairy",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("nuts"* OR "dairy"*)`,
			expectedIsNeg:     true,
		},
		{
			name:              "quoted phrase with negation",
			rawQuery:          `chocolate -"peanut butter"`,
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : (("chocolate"*) NOT "peanut butter")`,
		},
		{
			name:              "OR operator",
			rawQuery:          "apple OR banana",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("apple"* OR "banana"*)`,
		},
		{
			name:              "OR operator with negation",
			rawQuery:          "apple OR banana -walnuts",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : (("apple"* OR "banana"*) NOT "walnuts"*)`,
		},
		{
			name:              "OR followed by negated term does not bridge next term",
			rawQuery:          "apple OR -nuts banana",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : (("apple"* "banana"*) NOT "nuts"*)`,
		},
		{
			name:              "negated term followed by OR bridges positive terms",
			rawQuery:          "apple -nuts OR banana",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : (("apple"* OR "banana"*) NOT "nuts"*)`,
		},
		{
			name:              "stray OR operators stripped",
			rawQuery:          "OR apple OR OR banana OR",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("apple"* OR "banana"*)`,
		},
		{
			name:              "fractions and punctuation",
			rawQuery:          "recipe: 3/4 cup! *sweet*",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("recipe"* "3/4"* "cup"* "sweet"*)`,
		},
		{
			name:              "hyphenated words and contractions",
			rawQuery:          "all-purpose grandma's",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("all-purpose"* "grandma's"*)`,
		},
		{
			name:              "reserved FTS5 uppercase keywords treated as search words",
			rawQuery:          "mac AND cheese NOT ham",
			opts:              []fts5query.Option{fts5query.WithColumns("name")},
			expectedMatchExpr: `{name} : ("mac"* "AND"* "cheese"* "NOT"* "ham"*)`,
		},
		{
			name:            "all punctuation returns empty",
			rawQuery:        `: * ^ & { } "---"`,
			expectedIsEmpty: true,
		},
		{
			name:              "prefix matching disabled",
			rawQuery:          "chocolate chip",
			opts:              []fts5query.Option{fts5query.WithPrefixMatch(false)},
			expectedMatchExpr: `("chocolate" "chip")`,
		},
		{
			name:              "minimum term length filter",
			rawQuery:          "a big brown fox",
			opts:              []fts5query.Option{fts5query.WithMinTermLength(3)},
			expectedMatchExpr: `("big"* "brown"* "fox"*)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := fts5query.Parse(tc.rawQuery, tc.opts...)

			if res.IsEmpty() != tc.expectedIsEmpty {
				t.Fatalf("expected IsEmpty=%v, got %v", tc.expectedIsEmpty, res.IsEmpty())
			}
			if res.IsPureNegation != tc.expectedIsNeg {
				t.Errorf("expected IsPureNegation=%v, got %v", tc.expectedIsNeg, res.IsPureNegation)
			}
			if res.MatchExpr != tc.expectedMatchExpr {
				t.Errorf("expected MatchExpr %q, got %q", tc.expectedMatchExpr, res.MatchExpr)
			}

			if !res.IsEmpty() {
				rows, err := sqliteDB.Query("SELECT rowid FROM test_fts WHERE test_fts MATCH ?", res.MatchExpr)
				if err != nil {
					t.Errorf("FTS5 syntax error for MatchExpr %q: %v", res.MatchExpr, err)
				} else {
					_ = rows.Close()
				}
			}
		})
	}
}

func TestSQLCondition(t *testing.T) {
	// Standard positive query
	res := fts5query.Parse("cookies", fts5query.WithColumns("name"))
	stmt, args := res.SQLCondition("test_fts", "r.id")
	expectedStmt := "r.id IN (SELECT rowid FROM test_fts WHERE test_fts MATCH ?)"
	if stmt != expectedStmt {
		t.Errorf("expected stmt %q, got %q", expectedStmt, stmt)
	}
	if len(args) != 1 || args[0] != `{name} : ("cookies"*)` {
		t.Errorf("unexpected args: %v", args)
	}

	// Pure negation query
	resNeg := fts5query.Parse("-nuts", fts5query.WithColumns("name"))
	stmtNeg, argsNeg := resNeg.SQLCondition("test_fts", "r.id")
	expectedStmtNeg := "r.id NOT IN (SELECT rowid FROM test_fts WHERE test_fts MATCH ?)"
	if stmtNeg != expectedStmtNeg {
		t.Errorf("expected stmt %q, got %q", expectedStmtNeg, stmtNeg)
	}
	if len(argsNeg) != 1 || argsNeg[0] != `{name} : ("nuts"*)` {
		t.Errorf("unexpected args: %v", argsNeg)
	}

	// Empty query
	resEmpty := fts5query.Parse("")
	stmtEmpty, argsEmpty := resEmpty.SQLCondition("test_fts", "r.id")
	if stmtEmpty != "" || len(argsEmpty) != 0 {
		t.Errorf("expected empty condition, got stmt=%q, args=%v", stmtEmpty, argsEmpty)
	}
}
