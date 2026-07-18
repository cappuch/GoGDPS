package jsonl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Engine struct {
	dir       string
	mu        sync.RWMutex
	tables    map[string][]map[string]any
	sequences map[string]int
}

func OpenEngine(dir string) (*Engine, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	e := &Engine{
		dir:       dir,
		tables:    make(map[string][]map[string]any),
		sequences: make(map[string]int),
	}
	if err := e.loadAll(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Engine) loadAll() error {
	entries, err := os.ReadDir(e.dir)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".jsonl") {
			continue
		}
		table := strings.TrimSuffix(ent.Name(), ".jsonl")
		if err := e.loadTable(table); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) loadTable(name string) error {
	path := e.tablePath(name)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			e.tables[name] = nil
			return nil
		}
		return err
	}
	defer f.Close()

	var rows []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	e.tables[name] = rows
	e.refreshSequence(name)
	return nil
}

func (e *Engine) refreshSequence(table string) {
	pk := primaryKey(table)
	maxID := 0
	for _, row := range e.tables[table] {
		if v, ok := row[pk]; ok {
			if n, ok := toInt(v); ok && n > maxID {
				maxID = n
			}
		}
	}
	e.sequences[table] = maxID
}

func (e *Engine) saveTableLocked(name string) error {
	path := e.tablePath(name)
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, row := range e.tables[name] {
		if err := enc.Encode(row); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func (e *Engine) tablePath(name string) string {
	return filepath.Join(e.dir, name+".jsonl")
}

func primaryKey(table string) string {
	switch table {
	case "actions", "actions_downloads", "actions_likes", "bannedips", "blocks", "friendreqs", "modactions", "roleassign", "songs", "dailyfeatures", "gauntlets", "lists", "links", "quests", "reports", "suggest", "cpshares":
		return "ID"
	case "accounts":
		return "accountID"
	case "users":
		return "userID"
	case "levels":
		return "levelID"
	case "comments", "acccomments":
		return "commentID"
	case "messages":
		return "messageID"
	default:
		return "id"
	}
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		return i, err == nil
	default:
		return 0, false
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

func rowString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprint(v)
	}
}

func compareValues(a, b any) int {
	as := rowString(a)
	bs := rowString(b)
	if ai, ok := toInt64(a); ok {
		if bi, ok := toInt64(b); ok {
			switch {
			case ai < bi:
				return -1
			case ai > bi:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(as, bs)
}

func cloneRow(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		out[k] = v
	}
	return out
}

func sortRows(rows []map[string]any, order []orderClause) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, o := range order {
			ai, aok := rows[i][o.col]
			bj, bok := rows[j][o.col]
			if !aok || !bok {
				continue
			}
			cmp := compareValues(ai, bj)
			if cmp == 0 {
				continue
			}
			if o.desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}
