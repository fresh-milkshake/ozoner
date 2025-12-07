package parser

import (
	"strconv"
	"strings"
)

// Helper functions for extracting values from maps

func getString(m map[string]interface{}, keys ...string) (string, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case string:
				return v, true
			case float64:
				return strconv.FormatFloat(v, 'f', 0, 64), true
			case int:
				return strconv.Itoa(v), true
			}
		}
	}
	return "", false
}

func getFloat(m map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case float64:
				return v, true
			case int:
				return float64(v), true
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					return f, true
				}
			}
		}
	}
	return 0, false
}

func getInt(m map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case int:
				return v, true
			case float64:
				return int(v), true
			case string:
				if i, err := strconv.Atoi(v); err == nil {
					return i, true
				}
			}
		}
	}
	return 0, false
}

func getBool(m map[string]interface{}, keys ...string) (bool, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case bool:
				return v, true
			case string:
				return strings.ToLower(v) == "true" || v == "1", true
			case int:
				return v != 0, true
			}
		}
	}
	return false, false
}

func findItemsByPath(data map[string]interface{}, path []string) []interface{} {
	current := data
	for i, key := range path {
		if i == len(path)-1 {
			if val, ok := current[key]; ok {
				if items, ok := val.([]interface{}); ok {
					return items
				}
			}
		} else {
			if next, ok := current[key].(map[string]interface{}); ok {
				current = next
			} else {
				return nil
			}
		}
	}
	return nil
}

func findItemsInMap(m map[string]interface{}, keys []string) []interface{} {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if items, ok := val.([]interface{}); ok {
				return items
			}
		}
	}
	return nil
}

