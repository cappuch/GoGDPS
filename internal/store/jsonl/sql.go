package jsonl

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var engineRegistry sync.Map // dir -> *Engine

func init() {
	sql.Register("jsonl", &Driver{})
}

type Driver struct{}

func (d *Driver) Open(name string) (driver.Conn, error) {
	dir := strings.TrimPrefix(name, "file://")
	val, ok := engineRegistry.Load(dir)
	if !ok {
		e, err := OpenEngine(dir)
		if err != nil {
			return nil, err
		}
		actual, _ := engineRegistry.LoadOrStore(dir, e)
		val = actual
	}
	e, ok := val.(*Engine)
	if !ok {
		return nil, fmt.Errorf("jsonl: bad engine")
	}
	return &conn{engine: e}, nil
}

type conn struct {
	engine *Engine
	mu     sync.Mutex
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return &stmt{conn: c, query: query}, nil
}
func (c *conn) Close() error { return nil }
func (c *conn) Begin() (driver.Tx, error) { return &tx{c: c}, nil }

type tx struct{ c *conn }

func (t *tx) Commit() error   { return nil }
func (t *tx) Rollback() error { return nil }

type stmt struct {
	conn  *conn
	query string
}

func (s *stmt) Close() error { return nil }
func (s *stmt) NumInput() int {
	return strings.Count(s.query, "?")
}

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	return s.conn.engine.exec(s.query, valuesFromDriver(args))
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	return s.conn.engine.query(s.query, valuesFromDriver(args))
}

type result struct {
	affected int64
	lastID   int64
}

func (r result) LastInsertId() (int64, error) { return r.lastID, nil }
func (r result) RowsAffected() (int64, error) { return r.affected, nil }

type rows struct {
	cols   []string
	rows   []map[string]any
	idx    int
	closed bool
}

func (r *rows) Columns() []string { return r.cols }
func (r *rows) Close() error {
	r.closed = true
	return nil
}
func (r *rows) Next(dest []driver.Value) error {
	if r.closed || r.idx >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.idx]
	r.idx++
	for i, col := range r.cols {
		dest[i] = row[col]
	}
	return nil
}

func valuesFromDriver(args []driver.Value) []any {
	out := make([]any, len(args))
	for i, v := range args {
		out[i] = v
	}
	return out
}

var (
	errNoRows       = errors.New("sql: no rows in result set")
	errUnsupported  = errors.New("jsonl: unsupported query")
)

func (e *Engine) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exec(query, args)
}

func (e *Engine) QueryContext(ctx context.Context, query string, args ...any) (*rows, []string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, err := e.query(query, args)
	if err != nil {
		return nil, nil, err
	}
	return r, r.cols, nil
}

func (e *Engine) exec(query string, args []any) (result, error) {
	q := normalizeSQL(query)
	lq := lowerSQL(q)
	switch {
	case strings.HasPrefix(lq, "insert into"):
		return e.execInsert(lq, args)
	case strings.HasPrefix(lq, "update "):
		return e.execUpdate(lq, args)
	case strings.HasPrefix(lq, "delete from"):
		return e.execDelete(lq, args)
	default:
		return result{}, errUnsupported
	}
}

func (e *Engine) query(query string, args []any) (*rows, error) {
	q := normalizeSQL(query)
	if !strings.HasPrefix(lowerSQL(q), "select") {
		return nil, errUnsupported
	}
	plan, err := parseSelect(q)
	if err != nil {
		return nil, err
	}
	dataRows, err := e.runSelect(plan, args)
	if err != nil {
		return nil, err
	}
	return &rows{cols: plan.outCols, rows: dataRows}, nil
}

func normalizeSQL(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, "\n", " ")
	q = strings.ReplaceAll(q, "\t", " ")
	for strings.Contains(q, "  ") {
		q = strings.ReplaceAll(q, "  ", " ")
	}
	return q
}

func lowerSQL(q string) string {
	return strings.ToLower(q)
}

type orderClause struct {
	col  string
	desc bool
}

type selectPlan struct {
	countOnly bool
	countExpr string
	outCols   []string
	selectRaw []selectItem
	from      string
	joins     []joinClause
	where     string
	groupBy   string
	order     []orderClause
	limit     int
	offset    int
}

type selectItem struct {
	expr string
	alias string
}

type joinClause struct {
	joinType string
	table    string
	onLeft   string
	onRight  string
}

var (
	reSelectFrom = regexp.MustCompile(`^select (.+?) from (.+)$`)
	reLimit      = regexp.MustCompile(` limit (\?|\d+)(?: offset (\?|\d+))?`)
	reOrderBy    = regexp.MustCompile(` order by (.+?)(?: limit |$)`)
	reGroupBy    = regexp.MustCompile(` group by (.+?)(?: order by | limit |$)`)
	reWhere      = regexp.MustCompile(` where (.+?)(?: group by | order by | limit |$)`)
	reJoin       = regexp.MustCompile(` (left join|inner join|join) ([a-z0-9_.]+) on ([a-z0-9_.]+) = ([a-z0-9_.]+)`)
)

func parseSelect(q string) (*selectPlan, error) {
	plan := &selectPlan{limit: -1, offset: 0}
	work := lowerSQL(q)
	if m := reLimit.FindStringSubmatch(work); m != nil {
		plan.limit = parseLimitToken(m[1])
		if len(m) > 2 && m[2] != "" {
			plan.offset = parseLimitToken(m[2])
		}
		work = reLimit.ReplaceAllString(work, "")
	}
	if m := reOrderBy.FindStringSubmatch(work); m != nil {
		plan.order = parseOrder(m[1])
		work = reOrderBy.ReplaceAllString(work, "")
	}
	if m := reGroupBy.FindStringSubmatch(work); m != nil {
		plan.groupBy = strings.TrimSpace(m[1])
		work = reGroupBy.ReplaceAllString(work, "")
	}
	if m := reWhere.FindStringSubmatch(work); m != nil {
		plan.where = strings.TrimSpace(m[1])
		work = reWhere.ReplaceAllString(work, "")
	}
	for {
		m := reJoin.FindStringSubmatch(work)
		if m == nil {
			break
		}
		plan.joins = append(plan.joins, joinClause{
			joinType: strings.TrimSpace(m[1]),
			table:    tableName(m[2]),
			onLeft:   m[3],
			onRight:  m[4],
		})
		work = strings.Replace(work, m[0], "", 1)
	}
	m := reSelectFrom.FindStringSubmatch(lowerSQL(q))
	if m == nil {
		return nil, errUnsupported
	}
	plan.from = tableName(strings.TrimSpace(m[2]))
	items := splitSelectList(m[1])
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "count(*)" || strings.HasPrefix(it, "count(") {
			plan.countOnly = true
			plan.countExpr = it
			continue
		}
		alias := ""
		expr := it
		if idx := strings.LastIndex(it, " as "); idx >= 0 {
			expr = strings.TrimSpace(it[:idx])
			alias = strings.TrimSpace(it[idx+4:])
		}
		if alias == "" {
			if parts := strings.Split(expr, "."); len(parts) > 1 {
				alias = parts[len(parts)-1]
			} else {
				alias = expr
			}
		}
		plan.selectRaw = append(plan.selectRaw, selectItem{expr: expr, alias: alias})
		plan.outCols = append(plan.outCols, alias)
	}
	if plan.countOnly {
		plan.outCols = []string{"count(*)"}
	}
	return plan, nil
}

func parseLimitToken(tok string) int {
	if tok == "?" {
		return -2
	}
	n, _ := strconv.Atoi(tok)
	return n
}

func parseOrder(s string) []orderClause {
	parts := strings.Split(s, ",")
	var out []orderClause
	for _, p := range parts {
		p = strings.TrimSpace(p)
		desc := false
		if strings.HasSuffix(p, " desc") {
			desc = true
			p = strings.TrimSuffix(p, " desc")
		} else {
			p = strings.TrimSuffix(p, " asc")
		}
		p = strings.TrimSpace(p)
		if dot := strings.LastIndex(p, "."); dot >= 0 {
			p = p[dot+1:]
		}
		out = append(out, orderClause{col: p, desc: desc})
	}
	return out
}

func splitSelectList(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func tableName(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, " "); idx >= 0 {
		s = s[:idx]
	}
	if dot := strings.LastIndex(s, "."); dot >= 0 {
		s = s[dot+1:]
	}
	return s
}

func (e *Engine) runSelect(plan *selectPlan, args []any) ([]map[string]any, error) {
	base := e.materialize(plan.from)
	for _, j := range plan.joins {
		right := e.materialize(j.table)
		base = leftJoin(base, right, j.onLeft, j.onRight)
	}
	argIdx := 0
	filtered := make([]map[string]any, 0, len(base))
	for _, row := range base {
		ok, next, err := evalWhere(plan.where, row, args, argIdx)
		if err != nil {
			return nil, err
		}
		argIdx = next
		if ok {
			filtered = append(filtered, row)
		}
	}
	if plan.countOnly {
		if strings.Contains(plan.countExpr, "distinct") {
			col := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(plan.countExpr, "count(distinct "), ")"), " ")
			if dot := strings.LastIndex(col, "."); dot >= 0 {
				col = col[dot+1:]
			}
			seen := map[string]struct{}{}
			for _, row := range filtered {
				seen[rowString(row[col])] = struct{}{}
			}
			return []map[string]any{{"count(*)": len(seen)}}, nil
		}
		return []map[string]any{{"count(*)": len(filtered)}}, nil
	}
	out := make([]map[string]any, 0, len(filtered))
	for _, row := range filtered {
		item := map[string]any{}
		for _, sel := range plan.selectRaw {
			val, err := evalExpr(sel.expr, row, args)
			if err != nil {
				return nil, err
			}
			item[sel.alias] = val
		}
		out = append(out, item)
	}
	if len(plan.order) > 0 {
		sortRows(out, plan.order)
	}
	limit := plan.limit
	offset := plan.offset
	if limit == -2 {
		limit = toIntArg(args, argIdx)
		argIdx++
	}
	if offset == -2 {
		offset = toIntArg(args, argIdx)
		argIdx++
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(out) {
		out = nil
	} else if limit >= 0 {
		end := offset + limit
		if end > len(out) {
			end = len(out)
		}
		out = out[offset:end]
	} else if offset > 0 {
		out = out[offset:]
	}
	return out, nil
}

func toIntArg(args []any, idx int) int {
	if idx >= len(args) {
		return 0
	}
	n, _ := toInt(args[idx])
	return n
}

func (e *Engine) materialize(table string) []map[string]any {
	src := e.tables[table]
	out := make([]map[string]any, len(src))
	for i, row := range src {
		m := cloneRow(row)
		for k, v := range m {
			m[table+"."+k] = v
		}
		out[i] = m
	}
	return out
}

func leftJoin(left, right []map[string]any, onLeft, onRight string) []map[string]any {
	if len(left) == 0 {
		return nil
	}
	if len(right) == 0 {
		return left
	}
	var out []map[string]any
	for _, l := range left {
		lv, _ := evalExpr(onLeft, l, nil)
		matched := false
		for _, r := range right {
			rv, _ := evalExpr(onRight, r, nil)
			if rowString(lv) == rowString(rv) {
				merged := cloneRow(l)
				for k, v := range r {
					merged[k] = v
				}
				out = append(out, merged)
				matched = true
			}
		}
		if !matched {
			merged := cloneRow(l)
			for k := range right[0] {
				merged[k] = nil
			}
			out = append(out, merged)
		}
	}
	return out
}

func (e *Engine) execInsert(q string, args []any) (result, error) {
	// insert into table (cols) values (?,...)
	idx := strings.Index(q, "(")
	if idx < 0 {
		return result{}, errUnsupported
	}
	table := strings.TrimSpace(strings.TrimPrefix(q[:idx], "insert into"))
	colsPart := q[idx:]
	valIdx := strings.Index(strings.ToLower(colsPart), " values ")
	if valIdx < 0 {
		return result{}, errUnsupported
	}
	colsRaw := colsPart[1:valIdx]
	cols := splitCSV(strings.Trim(colsRaw, "()"))
	row := map[string]any{}
	argPos := 0
	for _, c := range cols {
		c = strings.TrimSpace(strings.Trim(c, "`"))
		if argPos >= len(args) {
			break
		}
		row[c] = args[argPos]
		argPos++
	}
	pk := primaryKey(table)
	if _, ok := row[pk]; !ok || rowString(row[pk]) == "" || rowString(row[pk]) == "0" {
		e.sequences[table]++
		row[pk] = e.sequences[table]
	}
	e.tables[table] = append(e.tables[table], row)
	if err := e.saveTableLocked(table); err != nil {
		return result{}, err
	}
	id, _ := toInt64(row[pk])
	return result{affected: 1, lastID: id}, nil
}

func (e *Engine) execUpdate(q string, args []any) (result, error) {
	// update table set col = ?, ... where ...
	rest := strings.TrimPrefix(q, "update ")
	setIdx := strings.Index(rest, " set ")
	if setIdx < 0 {
		return result{}, errUnsupported
	}
	table := tableName(rest[:setIdx])
	afterSet := rest[setIdx+6:]
	where := ""
	if idx := strings.Index(afterSet, " where "); idx >= 0 {
		where = strings.TrimSpace(afterSet[idx+7:])
		afterSet = afterSet[:idx]
	}
	sets := splitCSV(afterSet)
	argPos := 0
	updates := map[string]any{}
	for _, s := range sets {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			continue
		}
		col := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if val == "?" {
			if argPos < len(args) {
				updates[col] = args[argPos]
				argPos++
			}
		}
	}
	var affected int64
	rows := e.tables[table]
	for i, row := range rows {
		ok, next, err := evalWhere(where, row, args, argPos)
		if err != nil {
			return result{}, err
		}
		_ = next
		if !ok {
			continue
		}
		for k, v := range updates {
			row[k] = v
		}
		rows[i] = row
		affected++
	}
	e.tables[table] = rows
	if affected > 0 {
		if err := e.saveTableLocked(table); err != nil {
			return result{}, err
		}
	}
	return result{affected: affected}, nil
}

func (e *Engine) execDelete(q string, args []any) (result, error) {
	rest := strings.TrimPrefix(q, "delete from ")
	where := ""
	if idx := strings.Index(rest, " where "); idx >= 0 {
		where = strings.TrimSpace(rest[idx+7:])
		rest = rest[:idx]
	}
	table := tableName(rest)
	var kept []map[string]any
	var affected int64
	argPos := 0
	for _, row := range e.tables[table] {
		ok, next, err := evalWhere(where, row, args, argPos)
		if err != nil {
			return result{}, err
		}
		argPos = next
		if ok {
			affected++
			continue
		}
		kept = append(kept, row)
	}
	e.tables[table] = kept
	if affected > 0 {
		if err := e.saveTableLocked(table); err != nil {
			return result{}, err
		}
	}
	return result{affected: affected}, nil
}

func splitCSV(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}
