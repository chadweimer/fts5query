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
		{
			name:              "null bytes in unclosed quote",
			rawQuery:          "\"0\x00",
			expectedMatchExpr: `("0")`,
		},
		{
			name:              "null bytes within bare word",
			rawQuery:          "0\x000",
			expectedMatchExpr: `("0"* "0"*)`,
		},
		{
			name:              "null bytes in column names and values",
			rawQuery:          "\x00cookies\x00",
			opts:              []fts5query.Option{fts5query.WithColumns("name\x00", "\x00")},
			expectedMatchExpr: `{name} : ("cookies"*)`,
		},
		{
			name:            "null bytes only",
			rawQuery:        "\x00\x00",
			expectedIsEmpty: true,
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

	// Nil result
	var resNil *fts5query.Result
	stmtNil, argsNil := resNil.SQLCondition("test_fts", "r.id")
	if stmtNil != "" || len(argsNil) != 0 {
		t.Errorf("expected empty condition for nil, got stmt=%q, args=%v", stmtNil, argsNil)
	}
}

func TestSQLJoin(t *testing.T) {
	sqliteDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	defer func() { _ = sqliteDB.Close() }()

	_, err = sqliteDB.Exec(`
		CREATE TABLE recipe (id INTEGER PRIMARY KEY, name TEXT);
		CREATE VIRTUAL TABLE recipe_fts USING fts5(name, content='recipe', content_rowid='id');
		INSERT INTO recipe (id, name) VALUES (1, 'Chocolate Chip Cookies'), (2, 'Walnut Brownies');
		INSERT INTO recipe_fts (rowid, name) VALUES (1, 'Chocolate Chip Cookies'), (2, 'Walnut Brownies');
	`)
	if err != nil {
		t.Fatalf("failed to setup test tables: %v", err)
	}

	// 1. Standard positive query
	res := fts5query.Parse("cookies", fts5query.WithColumns("name"))
	join, where, args := res.SQLJoin("recipe_fts", "r.id")
	if join != "JOIN recipe_fts ON recipe_fts.rowid = r.id" {
		t.Errorf("unexpected join: %s", join)
	}
	if where != "recipe_fts MATCH ?" {
		t.Errorf("unexpected where: %s", where)
	}
	if len(args) != 1 || args[0] != `{name} : ("cookies"*)` {
		t.Errorf("unexpected args: %v", args)
	}

	rows, err := sqliteDB.Query("SELECT r.id FROM recipe AS r "+join+" WHERE "+where, args...)
	if err != nil {
		t.Fatalf("query execution failed: %v", err)
	}
	defer rows.Close()
	var matchedIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		matchedIDs = append(matchedIDs, id)
	}
	if rows.Err() != nil {
		t.Fatalf("rows iteration error: %v", rows.Err())
	}
	if len(matchedIDs) != 1 || matchedIDs[0] != 1 {
		t.Errorf("expected matched ID 1, got %v", matchedIDs)
	}

	// 2. Pure negation query
	resNeg := fts5query.Parse("-brownies", fts5query.WithColumns("name"))
	joinNeg, whereNeg, argsNeg := resNeg.SQLJoin("recipe_fts", "r.id")
	if joinNeg != "" {
		t.Errorf("expected empty join for pure negation, got: %s", joinNeg)
	}
	expectedWhereNeg := "r.id NOT IN (SELECT rowid FROM recipe_fts WHERE recipe_fts MATCH ?)"
	if whereNeg != expectedWhereNeg {
		t.Errorf("expected where %q, got %q", expectedWhereNeg, whereNeg)
	}
	if len(argsNeg) != 1 || argsNeg[0] != `{name} : ("brownies"*)` {
		t.Errorf("unexpected negation args: %v", argsNeg)
	}

	rowsNeg, err := sqliteDB.Query("SELECT r.id FROM recipe AS r WHERE "+whereNeg, argsNeg...)
	if err != nil {
		t.Fatalf("negation query execution failed: %v", err)
	}
	defer rowsNeg.Close()
	var matchedNegIDs []int
	for rowsNeg.Next() {
		var id int
		if err := rowsNeg.Scan(&id); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		matchedNegIDs = append(matchedNegIDs, id)
	}
	if rowsNeg.Err() != nil {
		t.Fatalf("rows iteration error: %v", rowsNeg.Err())
	}
	if len(matchedNegIDs) != 1 || matchedNegIDs[0] != 1 {
		t.Errorf("expected matched ID 1 excluding brownies, got %v", matchedNegIDs)
	}

	// 3. Empty query
	resEmpty := fts5query.Parse("")
	joinEmpty, whereEmpty, argsEmpty := resEmpty.SQLJoin("recipe_fts", "r.id")
	if joinEmpty != "" || whereEmpty != "" || len(argsEmpty) != 0 {
		t.Errorf("expected empty join/where/args, got join=%q, where=%q, args=%v", joinEmpty, whereEmpty, argsEmpty)
	}

	// 4. Nil result
	var resNil *fts5query.Result
	joinNil, whereNil, argsNil := resNil.SQLJoin("recipe_fts", "r.id")
	if joinNil != "" || whereNil != "" || len(argsNil) != 0 {
		t.Errorf("expected empty join/where/args for nil, got join=%q, where=%q, args=%v", joinNil, whereNil, argsNil)
	}
}

func TestBM25(t *testing.T) {
	sqliteDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	defer func() { _ = sqliteDB.Close() }()

	_, err = sqliteDB.Exec(`
		CREATE TABLE recipe (id INTEGER PRIMARY KEY, name TEXT);
		CREATE VIRTUAL TABLE recipe_fts USING fts5(name, content='recipe', content_rowid='id');
		INSERT INTO recipe (id, name) VALUES (1, 'Chocolate Chip Cookies');
		INSERT INTO recipe_fts (rowid, name) VALUES (1, 'Chocolate Chip Cookies');
	`)
	if err != nil {
		t.Fatalf("failed to setup test tables: %v", err)
	}

	res := fts5query.Parse("cookies", fts5query.WithColumns("name"))

	// With weights
	bm25WithWeights := res.BM25("recipe_fts", 10.0, 4.0, 2.0, 1.0, 1.0)
	if bm25WithWeights != "bm25(recipe_fts, 10, 4, 2, 1, 1)" {
		t.Errorf("unexpected bm25 with weights: %s", bm25WithWeights)
	}

	// Execute with weights
	queryWithWeights := "SELECT " + bm25WithWeights + " FROM recipe_fts WHERE recipe_fts MATCH ?"
	var score float64
	if err := sqliteDB.QueryRow(queryWithWeights, res.MatchExpr).Scan(&score); err != nil {
		t.Errorf("failed executing bm25 with weights query: %v", err)
	}

	// Without weights
	bm25Default := res.BM25("recipe_fts")
	if bm25Default != "bm25(recipe_fts)" {
		t.Errorf("unexpected default bm25: %s", bm25Default)
	}

	// Pure negation returns empty string
	resNeg := fts5query.Parse("-nuts")
	if s := resNeg.BM25("recipe_fts"); s != "" {
		t.Errorf("expected empty BM25 for pure negation, got: %s", s)
	}

	// Empty query returns empty string
	resEmpty := fts5query.Parse("")
	if s := resEmpty.BM25("recipe_fts"); s != "" {
		t.Errorf("expected empty BM25 for empty query, got: %s", s)
	}

	// Nil result returns empty string
	var resNil *fts5query.Result
	if s := resNil.BM25("recipe_fts"); s != "" {
		t.Errorf("expected empty BM25 for nil result, got: %s", s)
	}
}
