package jsonl

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func rowGet(row map[string]any, key string) (any, bool) {
	if v, ok := row[key]; ok {
		return v, true
	}
	lk := strings.ToLower(key)
	for k, v := range row {
		if strings.ToLower(k) == lk {
			return v, true
		}
	}
	return nil, false
}

func evalWhere(where string, row map[string]any, args []any, argIdx int) (bool, int, error) {
	if where == "" {
		return true, argIdx, nil
	}
	return evalBoolExpr(where, row, args, argIdx)
}

func evalBoolExpr(expr string, row map[string]any, args []any, argIdx int) (bool, int, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		return evalBoolExpr(expr[1:len(expr)-1], row, args, argIdx)
	}
	if idx := findTopLevel(expr, " or "); idx >= 0 {
		l, ai, err := evalBoolExpr(expr[:idx], row, args, argIdx)
		if err != nil {
			return false, ai, err
		}
		if l {
			return true, ai, nil
		}
		return evalBoolExpr(expr[idx+4:], row, args, ai)
	}
	if idx := findTopLevel(expr, " and "); idx >= 0 {
		l, ai, err := evalBoolExpr(expr[:idx], row, args, argIdx)
		if err != nil {
			return false, ai, err
		}
		if !l {
			return false, ai, nil
		}
		return evalBoolExpr(expr[idx+5:], row, args, ai)
	}
	if strings.HasPrefix(expr, "not ") {
		v, ai, err := evalBoolExpr(expr[4:], row, args, argIdx)
		return !v, ai, err
	}
	return evalComparison(expr, row, args, argIdx)
}

func findTopLevel(s, sep string) int {
	depth := 0
	for i := 0; i+len(sep) <= len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && strings.HasPrefix(s[i:], sep) {
			return i
		}
	}
	return -1
}

func evalComparison(expr string, row map[string]any, args []any, argIdx int) (bool, int, error) {
	if strings.Contains(expr, " in (") {
		return evalIn(expr, row, args, argIdx, false)
	}
	if strings.Contains(expr, " not in (") {
		return evalIn(strings.Replace(expr, " not in (", " in (", 1), row, args, argIdx, true)
	}
	ops := []string{" not like ", " like ", "!=", "<>", ">=", "<=", "=", ">", "<"}
	for _, op := range ops {
		if idx := strings.Index(expr, op); idx >= 0 {
			left := strings.TrimSpace(expr[:idx])
			right := strings.TrimSpace(expr[idx+len(op):])
			lv, err := evalExpr(left, row, args)
			if err != nil {
				return false, argIdx, err
			}
			var rv any
			if right == "?" {
				if argIdx >= len(args) {
					return false, argIdx, fmt.Errorf("missing arg")
				}
				rv = args[argIdx]
				argIdx++
			} else {
				rv, err = evalExpr(right, row, args)
				if err != nil {
					return false, argIdx, err
				}
			}
			switch strings.TrimSpace(op) {
			case "=", "is":
				return rowString(lv) == rowString(rv), argIdx, nil
			case "!=", "<>":
				return rowString(lv) != rowString(rv), argIdx, nil
			case ">":
				return compareValues(lv, rv) > 0, argIdx, nil
			case "<":
				return compareValues(lv, rv) < 0, argIdx, nil
			case ">=":
				return compareValues(lv, rv) >= 0, argIdx, nil
			case "<=":
				return compareValues(lv, rv) <= 0, argIdx, nil
			case "like":
				return sqlLike(rowString(lv), rowString(rv)), argIdx, nil
			case "not like":
				return !sqlLike(rowString(lv), rowString(rv)), argIdx, nil
			}
		}
	}
	if expr == "1" || expr == "true" {
		return true, argIdx, nil
	}
	v, err := evalExpr(expr, row, args)
	if err != nil {
		return false, argIdx, err
	}
	return truthy(v), argIdx, nil
}

func truthy(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	s := rowString(v)
	if s == "" || s == "0" || strings.EqualFold(s, "false") {
		return false
	}
	return true
}

func sqlLike(value, pattern string) bool {
	pattern = strings.ReplaceAll(regexp.QuoteMeta(pattern), "%", ".*")
	pattern = strings.ReplaceAll(pattern, "_", ".")
	ok, _ := regexp.MatchString("^"+pattern+"$", value)
	return ok
}

func evalExpr(expr string, row map[string]any, args []any) (any, error) {
	expr = strings.TrimSpace(expr)
	if expr == "?" {
		if len(args) == 0 {
			return nil, nil
		}
		return args[0], nil
	}
	if strings.HasPrefix(expr, "ifnull(") && strings.HasSuffix(expr, ")") {
		inner := expr[7 : len(expr)-1]
		parts := splitCSV(inner)
		if len(parts) < 2 {
			return nil, fmt.Errorf("ifnull")
		}
		v, err := evalExpr(parts[0], row, args)
		if err != nil {
			return nil, err
		}
		if v == nil || rowString(v) == "" {
			return evalExpr(parts[1], row, args)
		}
		return v, nil
	}
	if strings.HasPrefix(expr, "concat(") && strings.HasSuffix(expr, ")") {
		parts := splitCSV(expr[7 : len(expr)-1])
		var b strings.Builder
		for _, p := range parts {
			v, err := evalExpr(strings.TrimSpace(p), row, args)
			if err != nil {
				return nil, err
			}
			b.WriteString(rowString(v))
		}
		return b.String(), nil
	}
	if n, err := strconv.ParseInt(expr, 10, 64); err == nil {
		return n, nil
	}
	if strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'") {
		return strings.ReplaceAll(expr[1:len(expr)-1], "''", "'"), nil
	}
	if strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"") {
		return expr[1 : len(expr)-1], nil
	}
	if expr == "unix_timestamp()" || expr == "unix_timestamp" {
		return time.Now().Unix(), nil
	}
	if v, ok := rowGet(row, expr); ok {
		return v, nil
	}
	if strings.Contains(expr, ".") {
		if v, ok := rowGet(row, expr); ok {
			return v, nil
		}
		col := expr[strings.LastIndex(expr, ".")+1:]
		if v, ok := rowGet(row, col); ok {
			return v, nil
		}
	}
	if strings.HasPrefix(expr, "not ") {
		inner, err := evalExpr(expr[4:], row, args)
		if err != nil {
			return nil, err
		}
		return !truthy(inner), nil
	}
	return expr, nil
}

func evalIn(expr string, row map[string]any, args []any, argIdx int, invert bool) (bool, int, error) {
	idx := strings.Index(expr, " in (")
	left := strings.TrimSpace(expr[:idx])
	right := strings.TrimSuffix(strings.TrimPrefix(expr[idx+5:], "("), ")")
	lv, err := evalExpr(left, row, args)
	if err != nil {
		return false, argIdx, err
	}
	lvs := rowString(lv)
	var items []string
	if strings.TrimSpace(right) == "?" {
		if argIdx >= len(args) {
			return false, argIdx, fmt.Errorf("missing in arg")
		}
		items = strings.Split(rowString(args[argIdx]), ",")
		argIdx++
	} else {
		for _, p := range splitCSV(right) {
			p = strings.TrimSpace(p)
			p = strings.Trim(p, "'\"")
			items = append(items, p)
		}
	}
	found := false
	for _, it := range items {
		if rowString(it) == lvs {
			found = true
			break
		}
	}
	if invert {
		return !found, argIdx, nil
	}
	return found, argIdx, nil
}
