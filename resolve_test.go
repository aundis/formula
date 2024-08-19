package formula

import (
	"sort"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	examples := map[string][]string{
		"person.name + person.age + lala + run(a, b, c, d)":                                {"person.name", "person.age", "lala", "a", "b", "c", "d"},
		"age !== null ? '' : ($1=(name==='刚子'&&'刚子的年龄是必填的'),typeof $1 === 'string'?$1:'')": {"age", "name", "$1"},
	}

	for formula, except := range examples {
		code, err := ParseSourceCode([]byte(formula))
		if err != nil {
			t.Error(err)
			return
		}
		fields, err := ResolveReferenceFields(code)
		if err != nil {
			t.Error(err)
			return
		}
		if !stringsEquals(fields, except) {
			t.Errorf("parse formula (%s) except: %v, but got: %v", formula, except, fields)
			return
		}
	}
}

func TestResolveNotLocal(t *testing.T) {
	examples := map[string][]string{
		"age !== null ? '' : ($1=(name==='刚子'&&'刚子的年龄是必填的'),typeof $1 === 'string'?$1:'')": {"age", "name"},
	}

	for formula, except := range examples {
		code, err := ParseSourceCode([]byte(formula))
		if err != nil {
			t.Error(err)
			return
		}
		fields, err := ResolveReferenceFieldsNotLocal(code)
		if err != nil {
			t.Error(err)
			return
		}
		if !stringsEquals(fields, except) {
			t.Errorf("parse formula (%s) except: %v, but got: %v", formula, except, fields)
			return
		}
	}
}

func stringsEquals(arr1, arr2 []string) bool {
	if len(arr1) != len(arr2) {
		return false
	}
	sort.Slice(arr1, func(i, j int) bool { return arr1[i] < arr1[j] })
	sort.Slice(arr2, func(i, j int) bool { return arr2[i] < arr2[j] })
	return strings.Join(arr1, ",") == strings.Join(arr2, ",")
}
