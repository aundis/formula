package formula

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/ericlagergren/decimal"
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
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != float64(9) {
		t.Error("except 9 but got ", v)
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
	runner.SetThis(map[string]any{
		"value": []map[string]any{
			{
				"name": "小明",
			},
			{
				"name": "小红",
			},
			{
				"name": "小刚",
			},
		},
	})
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
func TestGetValue(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("person.age"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{
		"person": map[string]interface{}{
			"age": 18,
		},
	})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != float64(18) {
		t.Error("except person.age = 18 but got", v)
		return
	}
}

func TestEqualEqual(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("true == 1"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != true {
		t.Error("except true but got", v)
		return
	}
}

func TestOutput(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("find('hello world', 'o') + 10"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{
		"age": 18,
	})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(v)
}

func TestFunFinite(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("finite(a) + finite(b) + finite(c)"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{
		"a": math.NaN(),
		"b": math.Inf(1),
		"c": math.Inf(0),
	})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != float64(0) {
		t.Error("except 0 but got", v)
		return
	}
}

func TestGetObjectValueFromKey(t *testing.T) {
	// 使用一个不存在的key
	v, _ := getObjectValueFromKey(M{}, "a")
	if v != nil {
		t.Error("except nil")
		return
	}
	v, _ = getObjectValueFromKey(M{"age": 10}, "age")
	if v != 10 {
		t.Error("except 10")
		return
	}
}

func TestStringEqualsEqualsEqualsCmp(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("v==='染色'"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{
		"name": "染色",
	})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != false {
		t.Error("except false")
		return
	}
}

func TestFloatAdd(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("v+1.2"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{
		"v": 1,
	})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != float64(2.2) {
		t.Error("except 2.2")
		return
	}
}
func TestToString(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("toString(1)"))
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
	if v != "1" {
		t.Error("except '1'")
		return
	}
}

func TestToInt(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("toInt('1.3')"))
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
	if v != float64(1) {
		t.Error("except 1")
		return
	}
}

func TestToFloat(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("toFloat('5.5')"))
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
	if v != float64(5.5) {
		t.Error("except 5.5")
		return
	}
}

func TestCtxFunc(t *testing.T) {
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("add('1', 30)"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{
		"add": func(ctx context.Context, a string, b *decimal.Big) (string, error) {
			return fmt.Sprintf("%s,%s", a, b.String()), nil
		},
	})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != "1,30" {
		t.Error("except '1,30'")
		return
	}
}

func TestUseNilToArg(t *testing.T) {
	// nilInterface := reflect.New(reflect.TypeOf((*interface{})(nil)).Elem()).Elem().Interface()
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("finite(a)"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{
		"a": nil,
	})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != float64(0) {
		t.Error("except 0")
		return
	}
}

func TestNotNumber(t *testing.T) {
	// nilInterface := reflect.New(reflect.TypeOf((*interface{})(nil)).Elem()).Elem().Interface()
	ctx := context.Background()
	code, err := ParseSourceCode([]byte("!a"))
	if err != nil {
		t.Error(err)
		return
	}
	runner := NewRunner()
	runner.SetThis(map[string]interface{}{
		"a": 1,
	})
	v, err := runner.Resolve(ctx, code.Expression)
	if err != nil {
		t.Error(err)
		return
	}
	if v != false {
		t.Error("except false")
		return
	}
}
