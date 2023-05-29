package formula

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestConvTypeToTarget(t *testing.T) {
	var a [][]int32
	var target = reflect.TypeOf(a)
	r, err := convTypeToTarget([][]float32{{1.1, 2.2, 3.3}, {4.4, 5.5, 6.6}}, target)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%v", r)
}

func TestExpr(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("(1 + 2) * 3"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%T, %v", v, v)
}

func TestCallExpr(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("false ? upper(trim('abc')) : '啥东西'"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%T, %v", v, v)
}
