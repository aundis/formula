package formula

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const ctxKeyForRunner = "formulaRunner"

var callFunctions sync.Map

func init() {
	// FUNCTION TIME
	callFunctions.Store("now", funNow)
	callFunctions.Store("toDay", funToDay)
	callFunctions.Store("date", funDate)
	callFunctions.Store("addDate", funAddDate)
	callFunctions.Store("year", funYear)
	callFunctions.Store("month", funMonth)
	callFunctions.Store("day", funDay)
	callFunctions.Store("hour", funHour)
	callFunctions.Store("minute", funMinute)
	callFunctions.Store("second", funSecond)
	callFunctions.Store("millSecond", funMillSecond)
	callFunctions.Store("weekDay", funWeekDay)
	callFunctions.Store("timeFormat", funTimeFormat)
	// FUNCTION MATH
	callFunctions.Store("abs", funAbs)
	callFunctions.Store("ceil", funCeil)
	// runner.callFunctions.Store("min", funExp)
	callFunctions.Store("floor", funFloor)
	callFunctions.Store("ln", funLn)
	callFunctions.Store("log", funLog)
	callFunctions.Store("max", funMax)
	callFunctions.Store("min", funMin)
	callFunctions.Store("mod", funMod)
	callFunctions.Store("round", funRound)
	callFunctions.Store("roundBank", funRoundBank)
	callFunctions.Store("roundCash", funRoundCash)
	callFunctions.Store("roundCeil", funRoundCeil)
	callFunctions.Store("roundDown", funRoundDown)
	callFunctions.Store("roundFloor", funRoundFloor)
	callFunctions.Store("roundUp", funRoundUp)
	callFunctions.Store("sqrt", funSqrt)
	// FUNCTION STRING
	callFunctions.Store("startWith", funStartWith)
	callFunctions.Store("endWith", funEndWith)
	callFunctions.Store("contains", funContains)
	callFunctions.Store("find", funFind)
	callFunctions.Store("includes", funIncludes)
	callFunctions.Store("left", funLeft)
	callFunctions.Store("right", funRight)
	callFunctions.Store("len", funLen)
	callFunctions.Store("lower", funLower)
	callFunctions.Store("upper", funUpper)
	callFunctions.Store("lpad", funLpad)
	callFunctions.Store("rpad", funRpad)
	callFunctions.Store("mid", funMid)
	callFunctions.Store("replace", funReplace)
	callFunctions.Store("trim", funTrim)
	callFunctions.Store("regexp", funRegexp)
}

func NewRunner() *Runner {
	runner := &Runner{
		value: map[string]interface{}{},
	}
	return runner
}

func RunnerFromCtx(ctx context.Context) *Runner {
	if v := ctx.Value(ctxKeyForRunner); v != nil {
		return v.(*Runner)
	}
	return nil
}

type Runner struct {
	value                      map[string]interface{}
	IdentifierResolver         func(ctx context.Context, name string) (interface{}, error)
	SelectorExpressionResolver func(ctx context.Context, name string) (interface{}, error)
}

func (r *Runner) Resolve(ctx context.Context, v Expression) (interface{}, error) {
	switch n := v.(type) {
	case *Identifier:
		return r.resolveIdentifier(ctx, n)
	case *PrefixUnaryExpression:
		return r.resolvePrefixUnaryExpression(ctx, n)
	case *BinaryExpression:
		return r.resolveBinaryExpression(ctx, n)
	case *ArrayLiteralExpression:
		return r.resolveArrayLiteralExpression(ctx, n)
	case *ParenthesizedExpression:
		return r.resolveParenthesizedExpression(ctx, n)
	case *LiteralExpression:
		return r.resolveLiteralExpression(ctx, n)
	case *SelectorExpression:
		return r.resolveSelectorExpression(ctx, n)
	case *CallExpression:
		return r.resolveCallExpression(ctx, n)
	case *ConditionalExpression:
		return r.resolveConditionalExpression(ctx, n)
	default:
		return nil, errors.New("unknown expression type")
	}
}

func (r *Runner) resolveIdentifier(ctx context.Context, expr *Identifier) (interface{}, error) {
	switch expr.Value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		ctx = context.WithValue(ctx, ctxKeyForRunner, r)
		return r.IdentifierResolver(ctx, expr.Value)
	}
}

func (r *Runner) resolveSelectorExpression(ctx context.Context, expr *SelectorExpression) (interface{}, error) {
	names, err := resolveSelecotrNames(expr)
	if err != nil {
		return nil, err
	}
	name := strings.Join(names, ".")
	ctx = context.WithValue(ctx, ctxKeyForRunner, r)
	return r.SelectorExpressionResolver(ctx, name)
}

func resolveSelecotrNames(expr Expression) ([]string, error) {
	switch n := expr.(type) {
	case *SelectorExpression:
		arr, err := resolveSelecotrNames(n.Expression)
		if err != nil {
			return nil, err
		}
		return append(arr, n.Name.Value), nil
	case *Identifier:
		return []string{n.Value}, nil
	default:
		return nil, fmt.Errorf("selector name not support type %T", expr)
	}
}

func (r *Runner) resolveCallExpression(ctx context.Context, expr *CallExpression) (interface{}, error) {
	names, err := resolveCallNames(expr.Expression)
	if err != nil {
		return nil, err
	}
	name := strings.Join(names, ".")
	// 参数求值
	var args []interface{}
	if expr.Arguments != nil && expr.Arguments.Len() > 0 {
		for i := 0; i < expr.Arguments.Len(); i++ {
			av, err := r.Resolve(ctx, expr.Arguments.At(i))
			if err != nil {
				return nil, err
			}
			args = append(args, av)
		}
	}
	fun, ok := callFunctions.Load(name)
	if !ok {
		return nil, fmt.Errorf("not found call function '%s'", name)
	}
	funType := reflect.TypeOf(fun)
	hasVariadic := hasVariadicParameter(funType)
	// (...)可用性检查
	if expr.DotDotDotToken != nil && !hasVariadic {
		return nil, fmt.Errorf("call function '%s' error: not have variadic parammeter", name)
	}
	// 实参数量校验
	paramCount := funType.NumIn()
	if !hasVariadic || expr.DotDotDotToken != nil {
		if len(args) != paramCount-1 {
			return nil, fmt.Errorf("call function '%s' error: argument count except %d but got %d", name, paramCount-1, len(args))
		}
	} else {
		if len(args) < paramCount-2 {
			return nil, fmt.Errorf("call function '%s' error: argument count except greater than or equal %d but got %d", name, paramCount-2, len(args))
		}
	}
	// (...) 数组展开
	if len(args) > 0 && expr.DotDotDotToken != nil {
		expands, err := expandArrayArgument(args[len(args)-1])
		if err != nil {
			return nil, fmt.Errorf("call function '%s' error: %s", name, err.Error())
		}
		args = append(args[:len(args)-1], expands...)
	}
	// 参数转换
	callArgs := []reflect.Value{reflect.ValueOf(ctx)}
	for i := 0; i < len(args); i++ {
		var targetType reflect.Type
		if hasVariadic && i >= paramCount-2 {
			targetType = funType.In(paramCount - 1)
			targetType = targetType.Elem()
		} else {
			targetType = funType.In(i + 1)
		}
		convd, err := convTypeToTarget(args[i], targetType)
		if err != nil {
			return nil, fmt.Errorf("call function '%s' conv arg#%d error: %s", name, i+1, err.Error())
		}
		callArgs = append(callArgs, reflect.ValueOf(convd))
	}
	// 调用函数
	results := reflect.ValueOf(fun).Call(callArgs)
	if len(results) != 2 {
		return nil, fmt.Errorf("call function '%s' error: must return tow value but got %d", name, len(results))
	}
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
		err = fmt.Errorf("call function '%s' error: %s", name, err.Error())
	}
	return results[0].Interface(), err
}

func expandArrayArgument(v interface{}) ([]interface{}, error) {
	tpe := reflect.TypeOf(v)
	if tpe == nil {
		return nil, errors.New("expand array error: value type is nil")
	}
	if tpe.Kind() != reflect.Array && tpe.Kind() != reflect.Slice {
		return nil, fmt.Errorf("can't expand %T", v)
	}
	var result []interface{}
	value := reflect.ValueOf(v)
	for i := 0; i < value.Len(); i++ {
		result = append(result, value.Index(i).Interface())
	}
	return result, nil
}

func hasVariadicParameter(funType reflect.Type) bool {
	numArgs := funType.NumIn()
	if numArgs == 0 {
		return false
	}
	last := funType.In(numArgs - 1)
	return last != nil && last.Kind() == reflect.Slice
}

func convTypeToTarget(source interface{}, target reflect.Type) (interface{}, error) {
	switch target.Kind() {
	case reflect.Struct:
		return convStructToTarget(source, target)
	case reflect.String:
		return ToString(source)
	case reflect.Bool:
		return ToBool(source)
	case reflect.Array, reflect.Slice:
		return convArrayTypeToTarget(source, target)
	case reflect.Interface:
		return source, nil
	default:
		return nil, fmt.Errorf("convTypeToTarget not support type %v", target)
	}
}

func convStructToTarget(source interface{}, target reflect.Type) (interface{}, error) {
	if reflect.TypeOf(source) != target {
		return nil, fmt.Errorf("can't conv type %T to %T", source, target.String())
	}
	return source, nil
}

func convArrayTypeToTarget(source interface{}, target reflect.Type) (interface{}, error) {
	sourceValue := reflect.ValueOf(source)
	if sourceValue.Type() == nil || (sourceValue.Type().Kind() != reflect.Array && sourceValue.Type().Kind() != reflect.Slice) {
		return nil, fmt.Errorf("can't conv type %T to array", source)
	}
	sliceValue := reflect.MakeSlice(target, 0, 0)
	for i := 0; i < sourceValue.Len(); i++ {
		evalue, err := convTypeToTarget(sourceValue.Index(i).Interface(), target.Elem())
		if err != nil {
			return nil, err
		}
		sliceValue = reflect.Append(sliceValue, reflect.ValueOf(evalue))
	}
	return sliceValue.Interface(), nil
}

func resolveCallNames(expr Expression) ([]string, error) {
	switch n := expr.(type) {
	case *SelectorExpression:
		arr, err := resolveCallNames(n.Expression)
		if err != nil {
			return nil, err
		}
		return append(arr, n.Name.Value), nil
	case *Identifier:
		return []string{n.Value}, nil
	default:
		return nil, fmt.Errorf("call expression name not support type %T", expr)
	}
}

func (r *Runner) resolvePrefixUnaryExpression(ctx context.Context, expr *PrefixUnaryExpression) (interface{}, error) {
	v, err := r.Resolve(ctx, expr.Operand)
	if err != nil {
		return nil, err
	}
	switch expr.Operator.Token {
	case SK_Plus:
		return r.resolvePlusUnaryExpression(v)
	case SK_Minus:
		return r.resolveMinusUnaryExpression(v)
	case SK_Exclamation:
		return r.resolveExclamationUnaryExpression(v)
	case SK_Tilde:
		return r.resolveTildeUnaryExpression(v)
	}
	return nil, errors.New("unknown unary expression")
}

func (r *Runner) resolvePlusUnaryExpression(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case decimal.Decimal:
		return n, nil
	default:
		return nil, fmt.Errorf("unary expressin '+' not support type %T", v)
	}
}

func (r *Runner) resolveMinusUnaryExpression(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case decimal.Decimal:
		return n.Neg(), nil
	default:
		return nil, fmt.Errorf("unary expressin '-' not support type %T", v)
	}
}

func (r *Runner) resolveExclamationUnaryExpression(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case bool:
		return !n, nil
	default:
		return nil, fmt.Errorf("unary expressin '!' not support type %T", v)
	}
}

func (r *Runner) resolveTildeUnaryExpression(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case decimal.Decimal:
		return decimal.NewFromInt(^n.IntPart()), nil
	default:
		return nil, fmt.Errorf("unary expressin '~' not support type %T", v)
	}
}

func (r *Runner) resolveBinaryExpression(ctx context.Context, expr *BinaryExpression) (interface{}, error) {
	v1, err := r.Resolve(ctx, expr.Left)
	if err != nil {
		return nil, err
	}
	v2, err := r.Resolve(ctx, expr.Right)
	if err != nil {
		return nil, err
	}
	if reflect.TypeOf(v1) != reflect.TypeOf(v2) {
		return nil, fmt.Errorf("binary expression error: except expr1 type = expr2 type, but got %T !=  %T", v1, v2)
	}

	switch expr.Operator.Token {
	case SK_LessThan: // <
		return r.resolveLessThanBinaryExpressino(v1, v2)
	case SK_GreaterThan: // >
		return r.resolveGreaterThanBinaryExpressino(v1, v2)
	case SK_LessThanEquals: // <=
		return r.resolveLessThanEqualsBinaryExpressino(v1, v2)
	case SK_GreaterThanEquals: // >=
		return r.resolveGreaterThanEqualsBinaryExpressino(v1, v2)
	case SK_Plus: // +
		return r.resolvePlusBinaryExpression(v1, v2)
	case SK_Minus: // -
		return r.resolveMinusBinaryExpressino(v1, v2)
	case SK_Asterisk: // *
		return r.resolveAsteriskBinaryExpressino(v1, v2)
	case SK_Slash: // /
		return r.resolveSlashBinaryExpression(v1, v2)
	case SK_Percent: // %
		return r.resolvePercentBinaryExpression(v1, v2)
	case SK_Ampersand: // &
		return r.resolveAmpersandBinaryExpression(v1, v2)
	case SK_Bar: // |
		return r.resolveBarBinaryExpression(v1, v2)
	case SK_Caret: // ^
		return r.resolveCaretBinaryExpression(v1, v2)
	case SK_EqualsEquals: // ==
		return r.resolveEqualsEqualsBinaryExpression(expr, v1, v2)
	case SK_ExclamationEquals: // !
		return r.resolveNotEqualsBinaryExpression(expr, v1, v2)
	case SK_AmpersandAmpersand: // &&
		return r.resolveAmpersandAmpersandBinaryExpression(v1, v2)
	case SK_BarBar: // ||
		return r.resolveBarBarBinaryExpression(v1, v2)
	}
	return nil, nil
}

func (r *Runner) resolveLessThanBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).LessThan(v2.(decimal.Decimal)), nil
	default:
		return nil, fmt.Errorf("binary expression '<' not support type %T", v1)
	}
}

func (r *Runner) resolveGreaterThanBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).GreaterThan(v2.(decimal.Decimal)), nil
	default:
		return nil, fmt.Errorf("binary expressin '>' not support type %T", v1)
	}
}

func (r *Runner) resolveLessThanEqualsBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).LessThanOrEqual(v2.(decimal.Decimal)), nil
	default:
		return nil, fmt.Errorf("binary expression '<=' not support type %T", v1)
	}
}

func (r *Runner) resolveGreaterThanEqualsBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).GreaterThanOrEqual(v2.(decimal.Decimal)), nil
	default:
		return nil, fmt.Errorf("binary expression '>=' not support type %T", v1)
	}
}

func (r *Runner) resolvePlusBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).Add(v2.(decimal.Decimal)), nil
	case string:
		return v1.(string) + v2.(string), nil
	default:
		return nil, fmt.Errorf("binary expressin '+' not support type %T", v1)
	}
}

func (r *Runner) resolveMinusBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).Sub(v2.(decimal.Decimal)), nil
	default:
		return nil, fmt.Errorf("binary expression '-' not support type %T", v1)
	}
}

func (r *Runner) resolveAsteriskBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).Mul(v2.(decimal.Decimal)), nil
	default:
		return nil, fmt.Errorf("binary expression '*' not support type %T", v1)
	}
}

func (r *Runner) resolveSlashBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).Div(v2.(decimal.Decimal)), nil
	default:
		return nil, fmt.Errorf("binary expression '/' error: left or right expression type(%T) not support", v1)
	}
}

func (r *Runner) resolvePercentBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).Mod(v2.(decimal.Decimal)), nil
	default:
		return nil, fmt.Errorf("binary expression '%s' error: left or right expression type(%T) not support", "%", v1)
	}
}

func (r *Runner) resolveAmpersandBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		int1 := v1.(decimal.Decimal).IntPart()
		int2 := v2.(decimal.Decimal).IntPart()
		return decimal.NewFromInt(int1 & int2), nil
	default:
		return nil, fmt.Errorf("binary expression '&' error: left or right expression type(%T) not support", v1)
	}
}

func (r *Runner) resolveBarBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		int1 := v1.(decimal.Decimal).IntPart()
		int2 := v2.(decimal.Decimal).IntPart()
		return decimal.NewFromInt(int1 | int2), nil
	default:
		return nil, fmt.Errorf("binary expression '|' error: left or right expression type(%T) not support", v1)
	}
}

func (r *Runner) resolveCaretBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		int1 := v1.(decimal.Decimal).IntPart()
		int2 := v2.(decimal.Decimal).IntPart()
		return decimal.NewFromInt(int1 ^ int2), nil
	default:
		return nil, fmt.Errorf("binary expression '^' error: left or right expression type(%T) not support", v1)
	}
}

func (r *Runner) resolveEqualsEqualsBinaryExpression(expr *BinaryExpression, v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return v1.(decimal.Decimal).Equal(v2.(decimal.Decimal)), nil
	case bool:
		return v1.(bool) == v2.(bool), nil
	case string:
		return v1.(string) == v2.(string), nil
	default:
		return nil, fmt.Errorf("binary expression '!=' error: left or right expression type(%T) not support", v1)
	}
}

func (r *Runner) resolveNotEqualsBinaryExpression(expr *BinaryExpression, v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case decimal.Decimal:
		return !v1.(decimal.Decimal).Equal(v2.(decimal.Decimal)), nil
	case bool:
		return v1.(bool) != v2.(bool), nil
	case string:
		return v1.(string) != v2.(string), nil
	default:
		return nil, fmt.Errorf("binary expression '!=' error: left or right expression type(%T) not support", v1)
	}
}

func (r *Runner) resolveAmpersandAmpersandBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case bool:
		return v1.(bool) && v2.(bool), nil
	default:
		return nil, fmt.Errorf("binary expression '&&' error: left or right expression type(%T) not support", v1)
	}
}

func (r *Runner) resolveBarBarBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case bool:
		return v1.(bool) || v2.(bool), nil
	default:
		return nil, fmt.Errorf("binary expression '||' error: left or right expression type(%T) not support", v1)
	}
}

func (r *Runner) resolveArrayLiteralExpression(ctx context.Context, expr *ArrayLiteralExpression) (interface{}, error) {
	var list []interface{}
	if expr.Elements != nil && expr.Elements.Len() > 0 {
		for i := 0; i < expr.Elements.Len(); i++ {
			item := expr.Elements.At(i)
			v1, err := r.Resolve(ctx, item)
			if err != nil {
				return nil, err
			}
			list = append(list, v1)
		}
	}
	return list, nil
}

func (r *Runner) resolveParenthesizedExpression(ctx context.Context, expr *ParenthesizedExpression) (interface{}, error) {
	v, err := r.Resolve(ctx, expr.Expression)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *Runner) resolveLiteralExpression(ctx context.Context, expr *LiteralExpression) (interface{}, error) {
	switch expr.Token {
	case SK_NumberLiteral:
		return decimal.NewFromString(expr.Value)
	case SK_StringLiteral:
		return r.resolveStringLiteralExpression(expr)
	}
	return nil, errors.New("unknown liternal expression")
}

func (r *Runner) resolveStringLiteralExpression(expr *LiteralExpression) (interface{}, error) {
	return expr.Value, nil
}

func (r *Runner) resolveConditionalExpression(ctx context.Context, expr *ConditionalExpression) (interface{}, error) {
	cond, err := r.Resolve(ctx, expr.Condition)
	if err != nil {
		return nil, err
	}
	if !IsBoolean(cond) {
		return nil, errors.New("condition result value type not boolean")
	}
	if cond.(bool) {
		v, err := r.Resolve(ctx, expr.WhenTrue)
		if err != nil {
			return nil, err
		}
		return v, nil
	} else {
		v, err := r.Resolve(ctx, expr.WhenFalse)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
}

func (r *Runner) Set(key string, value interface{}) {
	r.value[key] = value
}

func (r *Runner) Get(key string) interface{} {
	return r.value[key]
}

// CALL

// FUNCTION DATE

func funNow(ctx context.Context) (time.Time, error) {
	return time.Now(), nil
}

func funToDay(ctx context.Context) (time.Time, error) {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local), nil
}

func funDate(ctx context.Context, y, m, d decimal.Decimal) (time.Time, error) {
	return time.Date(int(y.IntPart()), time.Month(int(m.IntPart())), int(d.IntPart()), 0, 0, 0, 0, time.Local), nil
}

func funAddDate(ctx context.Context, date time.Time, y, m, d decimal.Decimal) (time.Time, error) {
	return date.AddDate(int(y.IntPart()), int(m.IntPart()), int(d.IntPart())), nil
}

func funYear(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(date.Year())), nil
}

func funMonth(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(date.Month())), nil
}

func funDay(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(date.Day())), nil
}

func funHour(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(date.Hour())), nil
}

func funMinute(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(date.Minute())), nil
}

func funSecond(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(date.Second())), nil
}

func funMillSecond(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(date.UnixNano() / 1e6)), nil
}

func funWeekDay(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(int(date.Weekday()))), nil
}

func funTimeFormat(ctx context.Context, date time.Time, layout string) (string, error) {
	return date.Format(layout), nil
}

// FUNCTION MATH
func funAbs(ctx context.Context, v decimal.Decimal) (decimal.Decimal, error) {
	return v.Abs(), nil
}

func funCeil(ctx context.Context, v decimal.Decimal) (decimal.Decimal, error) {
	return v.Ceil(), nil
}

// func funExp(ctx context.Context, v decimal.Decimal) (decimal.Decimal, error) {
// 	return v.Exponent(), nil
// }

func funFloor(ctx context.Context, v decimal.Decimal) (decimal.Decimal, error) {
	return v.Floor(), nil
}

func funLn(ctx context.Context, v decimal.Decimal) (decimal.Decimal, error) {
	return decimal.NewFromFloat(math.Log(v.InexactFloat64())), nil
}

func funLog(ctx context.Context, v decimal.Decimal) (decimal.Decimal, error) {
	return decimal.NewFromFloat(math.Log10(v.InexactFloat64())), nil
}

func funMax(ctx context.Context, nums ...decimal.Decimal) (decimal.Decimal, error) {
	if len(nums) == 0 {
		return decimal.Zero, errors.New("please input numbers")
	}
	return decimal.Max(nums[0], nums[1:]...), nil
}

func funMin(ctx context.Context, nums ...decimal.Decimal) (decimal.Decimal, error) {
	if len(nums) == 0 {
		return decimal.Zero, errors.New("please input numbers")
	}
	return decimal.Min(nums[0], nums[1:]...), nil
}

func funMod(ctx context.Context, a, b decimal.Decimal) (decimal.Decimal, error) {
	return a.Mod(b), nil
}

func funRound(ctx context.Context, v, places decimal.Decimal) (decimal.Decimal, error) {
	return v.Round(int32(places.IntPart())), nil
}

func funRoundBank(ctx context.Context, v, places decimal.Decimal) (decimal.Decimal, error) {
	return v.RoundBank(int32(places.IntPart())), nil
}

func funRoundCash(ctx context.Context, v, places decimal.Decimal) (decimal.Decimal, error) {
	return v.RoundCash(uint8(places.IntPart())), nil
}

func funRoundCeil(ctx context.Context, v, places decimal.Decimal) (decimal.Decimal, error) {
	return v.RoundCeil(int32(places.IntPart())), nil
}

func funRoundDown(ctx context.Context, v, places decimal.Decimal) (decimal.Decimal, error) {
	return v.RoundDown(int32(places.IntPart())), nil
}

func funRoundFloor(ctx context.Context, v, places decimal.Decimal) (decimal.Decimal, error) {
	return v.RoundFloor(int32(places.IntPart())), nil
}

func funRoundUp(ctx context.Context, v, places decimal.Decimal) (decimal.Decimal, error) {
	return v.RoundUp(int32(places.IntPart())), nil
}

func funSqrt(ctx context.Context, v decimal.Decimal) (decimal.Decimal, error) {
	return decimal.NewFromFloat(math.Sqrt(v.InexactFloat64())), nil
}

// FUNCTION STRING
func funStartWith(ctx context.Context, s string, substr string) (bool, error) {
	return strings.Index(s, substr) == 0, nil
}

func funEndWith(ctx context.Context, s string, substr string) (bool, error) {
	return strings.Index(s, substr) == len(s)-len(substr), nil
}

func funContains(ctx context.Context, s string, substr string) (bool, error) {
	return strings.Contains(s, substr), nil
}

func funFind(ctx context.Context, s string, substr string) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(strings.Index(s, substr))), nil
}

func funIncludes(ctx context.Context, list []string, item string) (bool, error) {
	for _, v := range list {
		if v == item {
			return true, nil
		}
	}
	return false, nil
}

func funLeft(ctx context.Context, v string, ld decimal.Decimal) (string, error) {
	l := int(ld.IntPart())
	if l > len(v) {
		l = len(v)
	}
	return v[:l], nil
}

func funRight(ctx context.Context, v string, ld decimal.Decimal) (string, error) {
	l := int(ld.IntPart())
	if l > len(v) {
		l = len(v)
	}
	return v[len(v)-l:], nil
}

func funLen(ctx context.Context, v string) (decimal.Decimal, error) {
	return decimal.NewFromInt(int64(len(v))), nil
}

func funLower(ctx context.Context, v string) (string, error) {
	return strings.ToLower(v), nil
}

func funUpper(ctx context.Context, v string) (string, error) {
	return strings.ToUpper(v), nil
}

func funLpad(ctx context.Context, s, ps string, l decimal.Decimal) (string, error) {
	if len(s) > int(l.IntPart()) {
		return s[:int(l.IntPart())], nil
	}
	return strings.Repeat(ps, int(l.IntPart())-len(s)) + s, nil
}

func funRpad(ctx context.Context, s, ps string, l decimal.Decimal) (string, error) {
	if len(s) > int(l.IntPart()) {
		return s[:int(l.IntPart())], nil
	}
	return s + strings.Repeat(ps, int(l.IntPart())-len(s)), nil
}

func funMid(ctx context.Context, s string, start, end decimal.Decimal) (string, error) {
	istart := int(start.IntPart())
	iend := int(end.IntPart())
	if istart < 0 {
		istart = 0
	}
	if iend > len(s) {
		iend = len(s)
	}
	return s[istart:iend], nil
}

func funReplace(ctx context.Context, s, old, new string) (string, error) {
	return strings.ReplaceAll(s, old, new), nil
}

func funTrim(ctx context.Context, s string) (string, error) {
	return strings.TrimSpace(s), nil
}

func funRegexp(ctx context.Context, s string, reg string) (bool, error) {
	return regexp.MustCompile(reg).Match([]byte(s)), nil
}
