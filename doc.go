// Package fts5query translates human-typed web search queries into safe,
// robust SQLite FTS5 MATCH expressions and SQL query fragments.
//
// In systems like PostgreSQL, the built-in function `websearch_to_tsquery`
// converts messy user search inputs (such as space-separated terms, quoted
// phrases, negation with hyphens, and boolean ORs) into valid full-text queries
// without ever raising syntax errors on unclosed quotes or stray symbols.
//
// SQLite's FTS5 engine, by contrast, enforces a strict query grammar. Passing
// raw user input directly to `MATCH ?` causes runtime syntax errors
// (`fts5: syntax error`) whenever the input contains special FTS5 operators
// or punctuation (e.g., `:`, `*`, `^`, `{`, `}`, `(`, `)`, `"`, `-`, `/`, `!`,
// `'`, `.`) or uppercase reserved words (`AND`, `OR`, `NOT`).
//
// fts5query bridges this gap by parsing raw user queries, sanitizing terms,
// scoping column filters, properly parenthesizing boolean expressions to respect
// FTS5 operator precedence, and safely formatting bare words and quoted phrases.
// It also provides helpers for generating optimized SQL JOIN clauses (with automatic
// fallback for pure-negation queries) and bm25(...) relevance ranking expressions.
package fts5query
