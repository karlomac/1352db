package collate

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		name string
		a, b any
		want int
	}{
		{"nil nil", nil, nil, 0},
		{"nil vs bool", nil, true, -1},
		{"bool vs nil", true, nil, 1},
		{"false < true", false, true, -1},
		{"true == true", true, true, 0},
		{"false == false", false, false, 0},
		{"num less", 1.0, 2.0, -1},
		{"num greater", 2.0, 1.0, 1},
		{"num equal", 1.5, 1.5, 0},
		{"str less", "a", "b", -1},
		{"str greater", "b", "a", 1},
		{"str equal", "x", "x", 0},
		{"str prefix", "ab", "abc", -1},
		{"array less elem", []interface{}{1.0}, []interface{}{2.0}, -1},
		{"array equal", []interface{}{1.0, "a"}, []interface{}{1.0, "a"}, 0},
		{"array prefix", []interface{}{1.0}, []interface{}{1.0, 2.0}, -1},
		{"object less key", map[string]interface{}{"a": 1.0}, map[string]interface{}{"b": 1.0}, -1},
		{"object equal", map[string]interface{}{"a": 1.0}, map[string]interface{}{"a": 1.0}, 0},
		{"object value", map[string]interface{}{"a": 1.0}, map[string]interface{}{"a": 2.0}, -1},
		{"mixed nil < bool", nil, false, -1},
		{"mixed bool < num", false, 1.0, -1},
		{"mixed num < str", 1.0, "a", -1},
		{"mixed str < array", "a", []interface{}{}, -1},
		{"mixed array < object", []interface{}{}, map[string]interface{}{}, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Compare(c.a, c.b); got != c.want {
				t.Errorf("Compare(%v, %v) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b any
		want bool
	}{
		{"equal nil", nil, nil, true},
		{"nil vs value", nil, 1.0, false},
		{"equal bool", true, true, true},
		{"diff bool", true, false, false},
		{"equal num", 2.0, 2.0, true},
		{"equal str", "x", "x", true},
		{"equal array", []interface{}{1.0}, []interface{}{1.0}, true},
		{"diff array", []interface{}{1.0}, []interface{}{2.0}, false},
		{"equal object", map[string]interface{}{"a": 1.0}, map[string]interface{}{"a": 1.0}, true},
		{"mixed types", 1.0, "1", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Equal(c.a, c.b); got != c.want {
				t.Errorf("Equal(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestLess(t *testing.T) {
	cases := []struct {
		name string
		a, b any
		want bool
	}{
		{"nil < value", nil, 1.0, true},
		{"value < nil false", 1.0, nil, false},
		{"num less", 1.0, 2.0, true},
		{"num not less", 2.0, 1.0, false},
		{"equal not less", 1.0, 1.0, false},
		{"str less", "a", "b", true},
		{"mixed", false, 1.0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Less(c.a, c.b); got != c.want {
				t.Errorf("Less(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}
