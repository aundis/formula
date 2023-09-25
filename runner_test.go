package formula

import (
	"context"
	"reflect"
	"testing"
)

func TestConvTypeToTarget(t *testing.T) {
	var a [][]int32
	var target = reflect.TypeOf(a)
	_, err := convTypeToTarget([][]float32{{1.1, 2.2, 3.3}, {4.4, 5.5, 6.6}}, target)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestExpr(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("(1 + 2) * 3"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	_, err = runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestCallExpr(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("toDay()"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	_, err = runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestMapToArr(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("join(mapToArr(value, 'name'), ',')"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.IdentifierResolver = func(ctx context.Context, name string) (interface{}, error) {
		return []map[string]any{
			{
				"name": "小明",
			},
			{
				"name": "小红",
			},
			{
				"name": "小刚",
			},
		}, nil
	}
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != "小明,小红,小刚" {
		t.Errorf("except %s, but got %v", "小明,小红,小刚", v)
		return
	}
}
