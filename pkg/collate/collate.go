// Package collate implements the cdb/pdb collation (sorting) algorithm.
//
// This is a port of pdb-collate (https://github.com/pdb/pdb-collate),
// which itself mirrors cdb's Erlang collation. It is used to sort keys in
// views, indexes, and for checkpoint history comparison.
package collate

import (
	"sort"
	"strconv"
)

// Compare returns -1, 0, 1 comparing a and b using cdb collation.
func Compare(a, b any) int {
	return compare(a, b)
}

func compare(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	// null < boolean < number < string < array < object
	aType, bType := typeOrder(a), typeOrder(b)
	if aType != bType {
		if aType < bType {
			return -1
		}
		return 1
	}
	switch a := a.(type) {
	case bool:
		return boolCmp(a, b.(bool))
	case float64:
		return floatCmp(a, b.(float64))
	case string:
		return stringCmp(a, b.(string))
	case []interface{}:
		return arrayCmp(a, b.([]interface{}))
	case map[string]interface{}:
		return objectCmp(a, b.(map[string]interface{}))
	default:
		// fallback: stringify
		return stringCmp(toString(a), toString(b))
	}
}

func typeOrder(v any) int {
	switch v.(type) {
	case nil:
		return 0
	case bool:
		return 1
	case float64:
		return 2
	case string:
		return 3
	case []interface{}:
		return 4
	case map[string]interface{}:
		return 5
	default:
		return 6
	}
}

func boolCmp(a, b bool) int {
	if a == b {
		return 0
	}
	if !a {
		return -1
	}
	return 1
}

func floatCmp(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func stringCmp(a, b string) int {
	return stringCompare(a, b)
}

// stringCompare implements cdb's binary-ish UTF-8 comparison.
func stringCompare(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	n := len(ra)
	if len(rb) < n {
		n = len(rb)
	}
	for i := 0; i < n; i++ {
		if ra[i] < rb[i] {
			return -1
		}
		if ra[i] > rb[i] {
			return 1
		}
	}
	if len(ra) < len(rb) {
		return -1
	}
	if len(ra) > len(rb) {
		return 1
	}
	return 0
}

func arrayCmp(a, b []interface{}) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if c := compare(a[i], b[i]); c != 0 {
			return c
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func objectCmp(a, b map[string]interface{}) int {
	aKeys := sortedKeys(a)
	bKeys := sortedKeys(b)
	n := len(aKeys)
	if len(bKeys) < n {
		n = len(bKeys)
	}
	for i := 0; i < n; i++ {
		ka, kb := aKeys[i], bKeys[i]
		if ka != kb {
			if ka < kb {
				return -1
			}
			return 1
		}
		if c := compare(a[ka], b[kb]); c != 0 {
			return c
		}
	}
	if len(aKeys) < len(bKeys) {
		return -1
	}
	if len(aKeys) > len(bKeys) {
		return 1
	}
	return 0
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return ""
	}
}

// Less reports whether a < b in cdb collation.
func Less(a, b any) bool {
	return compare(a, b) < 0
}

// Equal reports whether a == b in cdb collation.
func Equal(a, b any) bool {
	return compare(a, b) == 0
}

// Max returns the larger of a and b.
func Max(a, b any) any {
	if compare(a, b) >= 0 {
		return a
	}
	return b
}

// Min returns the smaller of a and b.
func Min(a, b any) any {
	if compare(a, b) < 0 {
		return a
	}
	return b
}
