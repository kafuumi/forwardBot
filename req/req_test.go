package req

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestE_Json(t *testing.T) {
	tests := []struct {
		name string
		in   E
		out  string
	}{
		{"case type int", E{"number", 1}, `"number": 1`},
		{"case type float", E{"float", 2.1}, `"float": 2.1`},
		{"case type string", E{"demo", "ok"}, `"demo": "ok"`},
		{"case type nil", E{"nil", nil}, `"nil": null`},
		{"case type E", E{"element", E{"name", "value"}},
			`"element": {"name": "value"}`},
		{"case type D", E{"document", D{
			{"name0", 33},
			{"name1", "S"},
			{"name2", 1},
		}}, `"document": {"name0": 33,"name1": "S","name2": 1}`},
		{"case escape json", E{"enter", "\n\n"}, `"enter": "\n\n"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.out, test.in.Json())
		})
	}
}

func TestD_Json(t *testing.T) {
	tests := []struct {
		name string
		in   D
		out  string
	}{
		{"case 0", D{
			{"name", "value"},
			{"name1", 1},
			{"name2", 2},
		}, `{
    "name": "value",
    "name1": 1,
    "name2": 2
}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.out, test.in.Json())
		})
	}
}
