# fts5query

[![Go Reference](https://pkg.go.dev/badge/github.com/chadweimer/fts5query.svg)](https://pkg.go.dev/github.com/chadweimer/fts5query)
[![CI](https://github.com/chadweimer/fts5query/actions/workflows/ci.yml/badge.svg)](https://github.com/chadweimer/fts5query/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

**`fts5query`** translates human-typed search queries (Google/web-search style) into safe, optimal [SQLite FTS5](https://www.sqlite.org/fts5.html) `MATCH` expressions in pure Go with **zero runtime dependencies**.

It solves the SQLite FTS5 equivalent of PostgreSQL's [`websearch_to_tsquery`](https://www.postgresql.org/docs/current/textsearch-controls.html#TEXTSEARCH-PARSING-QUERIES), ensuring messy user input never triggers `fts5: syntax error` crashes while preserving expected search semantics.

---

## The Problem

In PostgreSQL, you can pass raw search strings directly to `websearch_to_tsquery('english', userInput)`, and it safely parses phrases, boolean `OR`s, and `-` negations without ever throwing syntax errors on unclosed quotes or symbols.

SQLite FTS5 does **not** have an equivalent built-in parser. Passing raw user input directly to `virtual_table MATCH ?` causes immediate runtime errors:

```text
fts5: syntax error near "/"   -- e.g. from "3/4 cup"
fts5: syntax error near "!"   -- e.g. from "sweet!"
fts5: syntax error near "AND" -- e.g. from "mac AND cheese"
```

Furthermore:
- In SQLite FTS5 grammar, column filters (`{col1 col2} : ...`) bind **only to the first token** unless parenthesized: `{title} : apple pie` parses as `({title} : apple) AND (ALL_COLUMNS : pie)`.
- FTS5 boolean `NOT` binds tighter than `OR`: `apple OR banana NOT walnuts` parses as `apple OR (banana NOT walnuts)`.
- FTS5 has no unary `NOT <word>` operator. Searching exclusively for `-walnuts` fails in FTS5 unless inverted at the SQL level.

`fts5query` addresses all of these caveats automatically.

---

## Features

- **Zero Runtime Dependencies**: Standard library only (`strings`, `unicode`, `fmt`).
- **Web Search Syntax**:
  - Implicit `AND` for space-separated words (`chocolate chip` $\rightarrow$ `"chocolate"* "chip"*`).
  - Quoted exact phrases (`"chocolate chip"` $\rightarrow$ `"chocolate chip"`).
  - Unclosed quotes handled gracefully (e.g. `"chocolate chip` treated as a phrase without error).
  - Prefix wildcard matching on words (`cookie` $\rightarrow$ `"cookie"*`) for natural autocomplete and stemming.
  - Logical `OR` (`apple OR banana`).
  - Negations with leading hyphen (`-walnuts` $\rightarrow$ `NOT "walnuts"*`).
- **Punctuation & Keyword Safety**: Words containing slashes (`3/4`), hyphens (`all-purpose`), contractions (`grandma's`), or uppercase keywords (`AND`, `NOT`) are safely quoted before wildcards (`"3/4"*`, `"all-purpose"*`) so SQLite's `unicode61` tokenizer splits them naturally.
- **Column Scoping & Precedence**: Correctly parenthesizes column filters (`{col1 col2} : (...)`) and mixed boolean expressions (`((pos1 OR pos2) NOT neg)`).
- **Pure Negation Handling**: Distinguishes pure-negation queries (e.g. `-nuts -dairy`) and provides SQL helpers to automatically emit `id NOT IN (SELECT rowid FROM fts WHERE fts MATCH ?)` with `OR`-joined exclusions.
- **Fuzz Tested**: Verified with continuous Go fuzz testing against in-memory SQLite FTS5 tables to guarantee no user input can cause a syntax error.

---

## Installation

```bash
go get github.com/chadweimer/fts5query
```

Requires Go 1.22 or higher.

---

## Quick Start

### Basic Usage

```go
package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/chadweimer/fts5query"
	_ "modernc.org/sqlite" // or github.com/mattn/go-sqlite3
)

func main() {
	db, err := sql.Open("sqlite", "recipes.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Parse user input from a search box
	userInput := `"chocolate chip" -nuts 3/4 cup! OR oatmeal`
	res := fts5query.Parse(userInput, fts5query.WithColumns("name", "ingredients"))

	if res.IsEmpty() {
		fmt.Println("No searchable terms entered.")
		return
	}

	fmt.Println("FTS5 MATCH Expression:", res.MatchExpr)
	// Output: {name ingredients} : ((("chocolate chip" ("3/4"* "cup"*) OR "oatmeal"*) NOT "nuts"*))

	// Execute via parameterized query
	rows, err := db.Query(`
		SELECT rowid, name 
		FROM recipe_fts 
		WHERE recipe_fts MATCH ? 
		ORDER BY rank
	`, res.MatchExpr)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// Process rows...
}
```

### Full SQL WHERE Condition Helper

When queries can contain pure negations (e.g. user types only `-dairy -nuts`), FTS5 requires matching the excluded terms and inverting the SQL condition using `NOT IN`:

```go
res := fts5query.Parse(userInput, fts5query.WithColumns("name", "ingredients"))

// Generates the appropriate IN or NOT IN clause with parameter bindings:
sqlCond, args := res.SQLCondition("recipe_fts", "recipe.id")
// For "cookies":
//   sqlCond: "recipe.id IN (SELECT rowid FROM recipe_fts WHERE recipe_fts MATCH ?)"
//   args:    []any{`{name ingredients} : ("cookies"*)`}
//
// For "-nuts":
//   sqlCond: "recipe.id NOT IN (SELECT rowid FROM recipe_fts WHERE recipe_fts MATCH ?)"
//   args:    []any{`{name ingredients} : ("nuts"*)`}

query := fmt.Sprintf("SELECT id, name FROM recipe WHERE %s ORDER BY name", sqlCond)
rows, err := db.Query(query, args...)
```

---

## Functional Options

Configure the parser using functional options:

```go
res := fts5query.Parse(
    userInput,
    // Restrict matches to specific virtual table columns
    fts5query.WithColumns("title", "body"),

    // Disable prefix wildcards on bare terms (default is true)
    fts5query.WithPrefixMatch(false),

    // Discard bare words shorter than N characters (default is 1)
    fts5query.WithMinTermLength(3),
)
```

---

## Comparison

| Feature | Raw SQLite FTS5 | `fts5query` | PostgreSQL `websearch_to_tsquery` |
| :--- | :---: | :---: | :---: |
| Unclosed quotes safe | ❌ Syntax Error | ✅ Handled | ✅ Handled |
| Internal punctuation safe (`3/4`, `grandma's`) | ❌ Syntax Error | ✅ Handled | ✅ Handled |
| Reserved keywords safe (`AND`, `NOT`) | ❌ Syntax Error | ✅ Handled | ✅ Handled |
| Word prefix matching | Manual `*` | ✅ Automatic | Manual `:*` |
| Exact quoted phrases | ✅ Supported | ✅ Supported | ✅ Supported |
| Leading hyphen negation (`-term`) | ❌ Syntax Error | ✅ Handled | ✅ Handled |
| Pure negation queries (`-nuts`) | ❌ Unsupported | ✅ SQL Inversion | ❌ No rows matched |
| Multi-column scoping | Manual | ✅ Automatic | N/A |
| Dependencies | N/A | **Zero** | Database built-in |

---

## License

MIT License. Copyright (c) 2026 Chad Weimer.
