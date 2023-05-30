package formula

import (
	"fmt"
	"testing"
)

func TestResolve(t *testing.T) {
	code, err := ParseSourceCode([]byte("person.name + person.age + lala + run(a, b, c, d)"))
	if err != nil {
		t.Error(err)
		return
	}
	fields, err := ResolveReferenceFields(code)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(fields)
}
