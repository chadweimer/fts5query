package fts5query

import (
	"fmt"
	"strconv"
	"strings"
)

// Result represents the parsed and sanitized FTS5 query output.
type Result struct {
	// MatchExpr is the sanitized FTS5 expression to pass to `MATCH ?`.
	// It is empty if the query contained no valid searchable terms.
	MatchExpr string

	// IsPureNegation is true if the query contained exclusively negative terms
	// (e.g. "-walnuts -peanuts"). Because SQLite FTS5's NOT operator is strictly
	// binary (<left> NOT <right>), pure-negation queries cannot be executed with
	// an FTS5 MATCH clause alone; they must be queried using a SQL NOT IN check
	// matching against the excluded terms.
	IsPureNegation bool

	// PositiveTerms contains the formatted positive terms, including logical OR operators.
	PositiveTerms []string

	// NegativeTerms contains the formatted negative terms to be excluded.
	NegativeTerms []string

	// Columns contains the target columns specified via WithColumns, if any.
	Columns []string
}

// IsEmpty returns true if the parsed query resulted in no searchable terms.
func (r *Result) IsEmpty() bool {
	return r == nil || r.MatchExpr == ""
}

// SQLCondition generates a parameterized SQL WHERE clause fragment and bind arguments.
// If the result is empty, it returns an empty statement and nil arguments.
//
// For pure negation queries (e.g. "-nuts"), it uses NOT IN:
//
//	"<primaryKeyColumn> NOT IN (SELECT rowid FROM <ftsTable> WHERE <ftsTable> MATCH ?)"
//
// For positive or mixed queries, it uses IN:
//
//	"<primaryKeyColumn> IN (SELECT rowid FROM <ftsTable> WHERE <ftsTable> MATCH ?)"
func (r *Result) SQLCondition(ftsTable, primaryKeyColumn string) (string, []any) {
	if r.IsEmpty() {
		return "", nil
	}
	op := "IN"
	if r.IsPureNegation {
		op = "NOT IN"
	}
	stmt := fmt.Sprintf("%s %s (SELECT rowid FROM %s WHERE %s MATCH ?)", primaryKeyColumn, op, ftsTable, ftsTable)
	return stmt, []any{r.MatchExpr}
}

// SQLJoin generates a JOIN clause, WHERE clause fragment, and bind arguments for querying an FTS5 virtual table.
//
// For positive or mixed queries:
//
//	joinClause:  "JOIN <ftsTable> ON <ftsTable>.rowid = <primaryKeyColumn>"
//	whereClause: "<ftsTable> MATCH ?"
//	args:        []any{r.MatchExpr}
//
// For pure negation queries (e.g. "-nuts"), which cannot be executed with an FTS5 MATCH clause alone:
//
//	joinClause:  ""
//	whereClause: "<primaryKeyColumn> NOT IN (SELECT rowid FROM <ftsTable> WHERE <ftsTable> MATCH ?)"
//	args:        []any{r.MatchExpr}
//
// For empty queries:
//
//	joinClause:  ""
//	whereClause: ""
//	args:        nil
func (r *Result) SQLJoin(ftsTable, primaryKeyColumn string) (string, string, []any) {
	if r.IsEmpty() {
		return "", "", nil
	}
	if r.IsPureNegation {
		cond, args := r.SQLCondition(ftsTable, primaryKeyColumn)
		return "", cond, args
	}
	join := fmt.Sprintf("JOIN %s ON %s.rowid = %s", ftsTable, ftsTable, primaryKeyColumn)
	where := fmt.Sprintf("%s MATCH ?", ftsTable)
	return join, where, []any{r.MatchExpr}
}

// BM25 returns an SQLite FTS5 `bm25(<ftsTable>, ...)` SQL expression for ranking search results by relevance.
//
// If optional weights are provided, they are formatted as comma-separated numeric arguments:
//
//	r.BM25("recipe_fts", 10.0, 4.0, 2.0, 1.0, 1.0)
//	// Returns: "bm25(recipe_fts, 10, 4, 2, 1, 1)"
//
// If the result is empty or represents a pure negation query (which cannot be scored via BM25),
// it returns an empty string.
func (r *Result) BM25(ftsTable string, weights ...float64) string {
	if r.IsEmpty() || r.IsPureNegation {
		return ""
	}
	if len(weights) == 0 {
		return fmt.Sprintf("bm25(%s)", ftsTable)
	}
	weightStrs := make([]string, len(weights))
	for i, w := range weights {
		weightStrs[i] = strconv.FormatFloat(w, 'f', -1, 64)
	}
	return fmt.Sprintf("bm25(%s, %s)", ftsTable, strings.Join(weightStrs, ", "))
}

// Parse transforms raw user-entered search text into a safe SQLite FTS5 query.
func Parse(rawQuery string, opts ...Option) *Result {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	sanitized := strings.ReplaceAll(rawQuery, "\x00", " ")
	trimmed := strings.TrimSpace(sanitized)
	if trimmed == "" {
		return &Result{Columns: cfg.columns}
	}

	posTerms, negTerms := parseTokens(trimmed, cfg)
	if len(posTerms) == 0 && len(negTerms) == 0 {
		return &Result{Columns: cfg.columns}
	}

	var colPrefix string
	if len(cfg.columns) > 0 {
		colPrefix = fmt.Sprintf("{%s} : ", strings.Join(cfg.columns, " "))
	}

	res := &Result{
		PositiveTerms: posTerms,
		NegativeTerms: negTerms,
		Columns:       cfg.columns,
	}

	// Case 1: Pure negation (e.g. "-walnuts -peanuts")
	if len(posTerms) == 0 {
		res.IsPureNegation = true
		res.MatchExpr = fmt.Sprintf("%s(%s)", colPrefix, strings.Join(negTerms, " OR "))
		return res
	}

	// Case 2: Positive terms, optionally combined with negative exclusions
	posExpr := strings.Join(posTerms, " ")
	if len(negTerms) > 0 {
		negExpr := strings.Join(negTerms, " NOT ")
		res.MatchExpr = fmt.Sprintf("%s((%s) NOT %s)", colPrefix, posExpr, negExpr)
	} else {
		res.MatchExpr = fmt.Sprintf("%s(%s)", colPrefix, posExpr)
	}

	return res
}
