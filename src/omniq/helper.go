package omniq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func QueueBase(queueName string) string {
	if containsHashTag(queueName) {
		return queueName
	}
	return "{" + queueName + "}"
}

func QueueAnchor(queueName string) string {
	return QueueBase(queueName) + ":meta"
}

func AsStr(v any) string {
	if v == nil {
		return ""
	}

	switch t := v.(type) {
	case []byte:
		return string(t)
	case string:
		return t
	default:
		return fmt.Sprint(v)
	}
}

func containsHashTag(s string) bool {
	hasOpen := false
	for _, r := range s {
		if r == '{' {
			hasOpen = true
		}
		if hasOpen && r == '}' {
			return true
		}
	}
	return false
}

func asAnySlice(v any) ([]any, bool) {
	switch t := v.(type) {
	case []any:
		return t, true
	default:
		return nil, false
	}
}

func toInt64(v any) (int64, error) {
	if v == nil {
		return 0, errors.New("nil")
	}
	switch t := v.(type) {
	case int:
		return int64(t), nil
	case int64:
		return t, nil
	case float64:
		return int64(t), nil
	case string:
		return strconv.ParseInt(strings.TrimSpace(t), 10, 64)
	case []byte:
		return strconv.ParseInt(strings.TrimSpace(string(t)), 10, 64)
	default:
		s := AsStr(t)
		return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	}
}

func toInt(v any) (int, error) {
	n, err := toInt64(v)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func isJSONStructured(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return false
	}

	switch v.(type) {
	case string, []byte:
		return false
	}

	k := rv.Kind()
	return k == reflect.Map || k == reflect.Slice || k == reflect.Array
}

func jsonCompactNoEscape(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		return "", err
	}

	b := bytes.TrimRight(buf.Bytes(), "\n")
	return string(b), nil
}

func splitKeysArgs(numkeys int, args []any) ([]string, []any) {
	if numkeys <= 0 {
		return nil, args
	}

	if numkeys > len(args) {
		// Defensive: if caller lies, treat all as argv.
		return nil, args
	}

	keys := make([]string, 0, numkeys)
	for i := 0; i < numkeys; i++ {
		keys = append(keys, AsStr(args[i]))
	}

	argv := args[numkeys:]
	return keys, argv
}