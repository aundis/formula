package formula

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ericlagergren/decimal"
)

const ctxKeyForRunner = "formulaRunner"

type M = map[string]interface{}

var innerMap sync.Map

func init() {
	// BOOL
	innerMap.Store("true", true)
	innerMap.Store("false", false)

	// FUNCTION TIME
	innerMap.Store("now", funNow)
	innerMap.Store("toDay", funToDay)
	innerMap.Store("date", funDate)
	innerMap.Store("addDate", funAddDate)
	innerMap.Store("year", funYear)
	innerMap.Store("month", funMonth)
	innerMap.Store("day", funDay)
	innerMap.Store("hour", funHour)
	innerMap.Store("minute", funMinute)
	innerMap.Store("second", funSecond)
	innerMap.Store("millSecond", funMillSecond)
	innerMap.Store("weekDay", funWeekDay)
	innerMap.Store("timeFormat", funTimeFormat)
	innerMap.Store("useTimezone", funUseTimezone)
	// FUNCTION MATH
	innerMap.Store("abs", funAbs)
	innerMap.Store("ceil", funCeil)
	innerMap.Store("exp", funExp)
	innerMap.Store("floor", funFloor)
	innerMap.Store("ln", funLn)
	innerMap.Store("log", funLog)
	innerMap.Store("max", funMax)
	innerMap.Store("min", funMin)
	innerMap.Store("round", funRound)
	innerMap.Store("roundBank", funRoundBank)
	innerMap.Store("roundCash", funRoundCash)
	innerMap.Store("sqrt", funSqrt)
	innerMap.Store("finite", funFinite)
	// FUNCTION STRING
	innerMap.Store("startWith", funStartWith)
	innerMap.Store("endWith", funEndWith)
	innerMap.Store("contains", funContains)
	innerMap.Store("find", funFind)
	innerMap.Store("includes", funIncludes)
	innerMap.Store("left", funLeft)
	innerMap.Store("right", funRight)
	innerMap.Store("len", funLen)
	innerMap.Store("lower", funLower)
	innerMap.Store("upper", funUpper)
	innerMap.Store("lpad", funLpad)
	innerMap.Store("rpad", funRpad)
	innerMap.Store("mid", funMid)
	innerMap.Store("replace", funReplace)
	innerMap.Store("trim", funTrim)
	innerMap.Store("regexp", funRegexp)
	// UTILITIES
	innerMap.Store("mapToArr", funMapToArr)
	innerMap.Store("join", funJoin)
	// CONV
	innerMap.Store("toString", funToString)
	innerMap.Store("toInt", funToInt)
	innerMap.Store("toFloat", funToFloat)

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
	this  map[string]interface{}
	value map[string]interface{}
}

func (r *Runner) SetThis(m map[string]interface{}) {
	r.this = m
}
func (r *Runner) SetThisValue(key string, value interface{}) {
	if r.this == nil {
		r.this = map[string]interface{}{}
	}
	r.this[key] = value
}

func (r *Runner) Resolve(ctx context.Context, v Expression) (interface{}, error) {
	res, err := r.resolve(ctx, v)
	if err != nil {
		return nil, err
	}
	return try2Float64(res), nil
}

func try2Float64(v interface{}) interface{} {
	switch n := v.(type) {
	case *decimal.Big:
		r, _ := n.Float64()
		return r
	}
	return v
}

func (r *Runner) resolve(ctx context.Context, v Expression) (res interface{}, err error) {
	switch n := v.(type) {
	case *Identifier:
		res, err = r.resolveIdentifier(ctx, n)
	case *PrefixUnaryExpression:
		res, err = r.resolvePrefixUnaryExpression(ctx, n)
	case *BinaryExpression:
		res, err = r.resolveBinaryExpression(ctx, n)
	case *ArrayLiteralExpression:
		res, err = r.resolveArrayLiteralExpression(ctx, n)
	case *ParenthesizedExpression:
		res, err = r.resolveParenthesizedExpression(ctx, n)
	case *LiteralExpression:
		res, err = r.resolveLiteralExpression(ctx, n)
	case *SelectorExpression:
		res, err = r.resolveSelectorExpression(ctx, n)
	case *CallExpression:
		res, err = r.resolveCallExpression(ctx, n)
	case *ConditionalExpression:
		res, err = r.resolveConditionalExpression(ctx, n)
	case *TypeOfExpression:
		res, err = r.resolveTypeofExpression(ctx, n)
	default:
		return nil, errors.New("unknown expression type")
	}
	if err != nil {
		return nil, err
	}
	return formatInput(res)
}

func formatInput(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case int:
		strconv.Itoa(n)
		return newDecimalBig().SetFloat64(float64(n)), nil
	case int32:
		return newDecimalBig().SetFloat64(float64(n)), nil
	case int64:
		return newDecimalBig().SetFloat64(float64(n)), nil
	case float32:
		// 避免精度损失
		nStr := strconv.FormatFloat(float64(n), 'f', -1, 64)
		r, _ := newDecimalBig().SetString(nStr)
		return r, nil
	case float64:
		// 避免精度损失
		nStr := strconv.FormatFloat(float64(n), 'f', -1, 64)
		r, _ := newDecimalBig().SetString(nStr)
		return r, nil
	case time.Time:
		return n, nil
	case string:
		return n, nil
	case bool:
		return n, nil
	case nil:
		return nil, nil
	default:
		return n, nil
	}
}

func (r *Runner) resolveIdentifier(ctx context.Context, expr *Identifier) (interface{}, error) {
	if v, ok := innerMap.Load(expr.Value); ok {
		return v, nil
	}
	return r.this[expr.Value], nil
}

func (r *Runner) resolveSelectorExpression(ctx context.Context, expr *SelectorExpression) (interface{}, error) {
	v, err := r.resolve(ctx, expr.Expression)
	if err != nil {
		return nil, err
	}
	if IsNull(v) && expr.Assert {
		return nil, fmt.Errorf("expr %s value is null, can't access attribute '%s'", astToString(expr.Expression), expr.Name.Value)
	}

	value, err := getObjectValueFromKey(v, expr.Name.Value)
	if err != nil {
		return nil, err
	}
	// 统一nil值
	return formatNilValue(value), nil
}

func formatNilValue(v any) any {
	if IsNull(v) {
		return nil
	}
	return v
}

func getObjectValueFromKey(v interface{}, key string) (interface{}, error) {
	if IsNull(v) {
		return nil, nil
	}
	rt := reflect.TypeOf(v)
	rv := reflect.ValueOf(v)
	switch rt.Kind() {
	case reflect.Map:
		mv := rv.MapIndex(reflect.ValueOf(key))
		if mv.Kind() == 0 || mv.IsZero() {
			return nil, nil
		}
		return mv.Interface(), nil
		// return rv.MapIndex(reflect.ValueOf(key)).Interface(), nil
	case reflect.Struct:
		field := rv.FieldByName(key)
		return field.Interface(), nil
	}
	return nil, nil
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
	fun, err := r.resolve(ctx, expr.Expression)
	if err != nil {
		return nil, err
	}
	names, err := resolveCallNames(expr.Expression)
	if err != nil {
		return nil, err
	}
	name := strings.Join(names, ".")
	// 参数求值
	var args []interface{}
	if expr.Arguments != nil && expr.Arguments.Len() > 0 {
		for i := 0; i < expr.Arguments.Len(); i++ {
			av, err := r.resolve(ctx, expr.Arguments.At(i))
			if err != nil {
				return nil, err
			}
			args = append(args, av)
		}
	}
	funType := reflect.TypeOf(fun)
	if funType.Kind() != reflect.Func {
		return nil, fmt.Errorf("expr %s value not is function", name)
	}
	hasVariadic := hasVariadicParameter(funType)
	// (...)可用性检查
	if expr.DotDotDotToken != nil && !hasVariadic {
		return nil, fmt.Errorf("call function '%s' error: not have variadic parammeter", name)
	}
	// 实参数量校验
	paramCount := funType.NumIn()
	// 最少要传递的参数个数
	minArgsCount := paramCount
	hasContextParam := 0
	if firstParamIsContext(funType) {
		hasContextParam = 1
	}
	if hasContextParam == 1 {
		minArgsCount--
	}
	if !hasVariadic || expr.DotDotDotToken != nil {
		if len(args) != minArgsCount {
			return nil, fmt.Errorf("call function '%s' error: argument count except %d but got %d", name, minArgsCount, len(args))
		}
	} else {
		if len(args) < minArgsCount-1 {
			return nil, fmt.Errorf("call function '%s' error: argument count except greater than or equal %d but got %d", name, minArgsCount-1, len(args))
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
	callArgs := []reflect.Value{}
	if hasContextParam == 1 {
		callArgs = append(callArgs, reflect.ValueOf(ctx))
	}
	for i := 0; i < len(args); i++ {
		var targetType reflect.Type
		if hasVariadic && i >= minArgsCount-1 {
			targetType = funType.In(paramCount - 1)
			targetType = targetType.Elem()
		} else {
			targetType = funType.In(i + hasContextParam)
		}
		convd, err := convTypeToTarget(args[i], targetType)
		if err != nil {
			return nil, fmt.Errorf("call function '%s' conv arg#%d error: %s", name, i+1, err.Error())
		}
		if convd == nil {
			// 根据参数类型创建对应类型的零值
			nilValue := reflect.Zero(targetType)
			callArgs = append(callArgs, reflect.ValueOf(nilValue))
		} else {
			callArgs = append(callArgs, reflect.ValueOf(convd))
		}
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

func firstParamIsContext(funcType reflect.Type) bool {
	if funcType.NumIn() > 0 {
		// 获取第一个参数的类型
		firstParamType := funcType.In(0)
		// 检查第一个参数的类型是否为 context.Context
		return firstParamType == reflect.TypeOf((*context.Context)(nil)).Elem()
	}
	return false
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
	case reflect.Interface:
		return source, nil
	case reflect.Array, reflect.Slice:
		return convArrayTypeToTarget(source, target)
	case reflect.Struct:
		return convStructToTarget(source, target)
	case reflect.Map:
		return convMapToTarget(source, target)
	default:
		if source != nil {
			rv := reflect.ValueOf(source)
			if rv.IsValid() && rv.CanConvert(target) {
				return rv.Convert(target).Interface(), nil
			}
		}
		if isBasicNumberKind(target.Kind()) {
			return convToBasicNumber(source, target)
		}
		if target.Kind() == reflect.String {
			if IsNull(source) {
				return "", nil
			}
			return fmt.Sprintf("%v", source), nil
		}
		return nil, fmt.Errorf("convTypeToTarget %T not conv to %v", source, target)
	}
}

var basicNumberKind = []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int, reflect.Float32, reflect.Float64}

func isBasicNumberKind(kind reflect.Kind) bool {
	for _, k := range basicNumberKind {
		if k == kind {
			return true
		}
	}
	return false
}

func convToBasicNumber(source interface{}, target reflect.Type) (interface{}, error) {
	if v, ok := source.(*decimal.Big); ok {
		f, _ := v.Float64()
		switch target.Kind() {
		case reflect.Int8:
			return int8(f), nil
		case reflect.Int16:
			return int16(f), nil
		case reflect.Int:
			return int(f), nil
		case reflect.Int32:
			return int32(f), nil
		case reflect.Int64:
			return int64(f), nil
		case reflect.Float32:
			return float32(f), nil
		case reflect.Float64:
			return float64(f), nil
		default:
			return nil, fmt.Errorf("convToBasicNumber %v not number target", target)
		}
	}
	return nil, fmt.Errorf("convToBasicNumber %T not decimal.Big", source)
}

func convStructToTarget(source interface{}, target reflect.Type) (interface{}, error) {
	if reflect.TypeOf(source) != target {
		return nil, fmt.Errorf("can't conv type %T to %T", source, target.String())
	}
	return source, nil
}

func convMapToTarget(source interface{}, target reflect.Type) (interface{}, error) {
	st := reflect.TypeOf(source)
	if st.Key() != target.Key() {
		return nil, fmt.Errorf("convMapToTarget error map key type %T != %T", st.Key(), target.Key())
	}

	sv := reflect.ValueOf(source)
	result := reflect.MakeMap(target)
	iter := sv.MapRange()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		evalue, err := convTypeToTarget(v.Interface(), target.Elem())
		if err != nil {
			return nil, err
		}
		result.SetMapIndex(k, reflect.ValueOf(evalue))
	}
	return result.Interface(), nil
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
	v, err := r.resolve(ctx, expr.Operand)
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
	case SK_ExclamationExclamation:
		return r.resolveExclamationExclamationUnaryExpression(v)
	}
	return nil, errors.New("unknown unary expression")
}

func (r *Runner) resolvePlusUnaryExpression(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case *decimal.Big:
		return n, nil
	case string:
		r, err := strconv.Atoi(n)
		if err != nil {
			return math.NaN(), nil
		}
		return r, nil
	case map[string]any:
		return math.NaN(), nil
	default:
		return nil, fmt.Errorf("unary expressin '+' not support type %T", v)
	}
}

func (r *Runner) resolveMinusUnaryExpression(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case *decimal.Big:
		return newDecimalBig().Neg(n), nil
	case string:
		r, err := strconv.Atoi(n)
		if err != nil {
			return math.NaN(), nil
		}
		return -r, nil
	case map[string]any:
		return math.NaN(), nil
	default:
		return nil, fmt.Errorf("unary expressin '-' not support type %T", v)
	}
}

func (r *Runner) resolveExclamationUnaryExpression(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case bool:
		return !n, nil
	case *decimal.Big:
		return !r.toBool(n), nil
	case nil:
		return true, nil
	default:
		return nil, fmt.Errorf("unary expressin '!' not support type %T", v)
	}
}

func (r *Runner) resolveExclamationExclamationUnaryExpression(v interface{}) (interface{}, error) {
	return r.toBool(v), nil
}

func (r *Runner) resolveTildeUnaryExpression(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case *decimal.Big:
		iv, _ := n.Int64()
		return newDecimalBig().SetUint64(uint64(iv)), nil
	default:
		return nil, fmt.Errorf("unary expressin '~' not support type %T", v)
	}
}

func (r *Runner) resolveBinaryExpression(ctx context.Context, expr *BinaryExpression) (interface{}, error) {
	// First process assignment expression
	switch expr.Operator.Token {
	case SK_Equals:
		return r.resolveEqualBinaryExpression(ctx, expr.Left, expr.Right)
	}

	v1, err := r.resolve(ctx, expr.Left)
	if err != nil {
		return nil, err
	}
	v2, err := r.resolve(ctx, expr.Right)
	if err != nil {
		return nil, err
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
	case SK_ExclamationEquals: // !=
		return r.resolveNotEqualsBinaryExpression(expr, v1, v2)
	case SK_EqualsEqualsEquals:
		return r.resolveEqualsEqualsEqualsBinaryExpression(expr, v1, v2)
	case SK_ExclamationEqualsEquals:
		return r.resolveNotEqualsEqualsBinaryExpression(expr, v1, v2)
	case SK_AmpersandAmpersand: // &&
		return r.resolveAmpersandAmpersandBinaryExpression(v1, v2)
	case SK_BarBar: // ||
		return r.resolveBarBarBinaryExpression(v1, v2)
	case SK_Comma:
		return r.resolveCommaBinaryExpression(v1, v2)
	}
	return nil, nil
}

func (r *Runner) resolveLessThanBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case string:
		s1 := convToString(v1)
		s2 := convToString(v2)
		return s1 < s2, nil
	default:
		n1 := convToNumber(v1)
		n2 := convToNumber(v2)
		return n1.Cmp(n2) == -1, nil
	}
}

func (r *Runner) resolveGreaterThanBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case string:
		s1 := convToString(v1)
		s2 := convToString(v2)
		return s1 > s2, nil
	default:
		n1 := convToNumber(v1)
		n2 := convToNumber(v2)
		return n1.Cmp(n2) == 1, nil
	}
}

func (r *Runner) resolveLessThanEqualsBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case string:
		s1 := convToString(v1)
		s2 := convToString(v2)
		return s1 <= s2, nil
	default:
		n1 := convToNumber(v1)
		n2 := convToNumber(v2)
		return n1.Cmp(n2) <= 0, nil
	}
}

func (r *Runner) resolveGreaterThanEqualsBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case string:
		s1 := convToString(v1)
		s2 := convToString(v2)
		return s1 >= s2, nil
	default:
		n1 := convToNumber(v1)
		n2 := convToNumber(v2)
		return n1.Cmp(n2) >= 0, nil
	}
}

func (r *Runner) resolvePlusBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case string:
		s1 := convToString(v1)
		s2 := convToString(v2)
		return s1 + s2, nil
	default:
		n1 := convToNumber(v1)
		n2 := convToNumber(v2)
		return newDecimalBig().Add(n1, n2), nil
	}
}

func (r *Runner) resolveMinusBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	switch v1.(type) {
	case string:
		s1 := convToString(v1)
		s2 := convToString(v2)
		return s1 + s2, nil
	default:
		n1 := convToNumber(v1)
		n2 := convToNumber(v2)
		return newDecimalBig().Sub(n1, n2), nil
	}
}

func (r *Runner) resolveAsteriskBinaryExpressino(v1, v2 interface{}) (interface{}, error) {
	n1 := convToNumber(v1)
	n2 := convToNumber(v2)
	return newDecimalBig().Mul(n1, n2), nil
}

func (r *Runner) resolveSlashBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	n1 := convToNumber(v1)
	n2 := convToNumber(v2)
	return newDecimalBig().Quo(n1, n2), nil
}

func (r *Runner) resolvePercentBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	n1 := convToNumber(v1)
	n2 := convToNumber(v2)
	return newDecimalBig().Rem(n1, n2), nil
}

func (r *Runner) resolveAmpersandBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	i1, _ := convToNumber(v1).Int64()
	i2, _ := convToNumber(v2).Int64()
	return newDecimalBig().SetFloat64(float64(i1 & i2)), nil
}

func (r *Runner) resolveBarBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	i1, _ := convToNumber(v1).Int64()
	i2, _ := convToNumber(v2).Int64()
	return newDecimalBig().SetFloat64(float64(i1 | i2)), nil
}

func (r *Runner) resolveCaretBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	i1, _ := convToNumber(v1).Int64()
	i2, _ := convToNumber(v2).Int64()
	return newDecimalBig().SetFloat64(float64(i1 ^ i2)), nil
}

func (r *Runner) resolveEqualsEqualsBinaryExpression(expr *BinaryExpression, v1, v2 interface{}) (interface{}, error) {
	return r.valueLikeEqualTo(v1, v2), nil
}

func (r *Runner) resolveNotEqualsBinaryExpression(expr *BinaryExpression, v1, v2 interface{}) (interface{}, error) {
	return !r.valueLikeEqualTo(v1, v2), nil
}

func (r *Runner) valueLikeEqualTo(v1, v2 interface{}) bool {
	switch v1.(type) {
	case *decimal.Big:
		n1 := convToNumber(v1)
		n2 := convToNumber(v2)
		return n1.Cmp(n2) == 0
	case bool:
		n1 := convToNumber(v1)
		n2 := convToNumber(v2)
		return n1.Cmp(n2) == 0
	case string:
		s1 := convToString(v1)
		s2 := convToString(v2)
		return s1 == s2
	default:
		return IsNull(v1) && IsNull(v2) || v1 == v2
	}
}

func (r *Runner) resolveEqualsEqualsEqualsBinaryExpression(expr *BinaryExpression, v1, v2 interface{}) (interface{}, error) {
	return r.valueEqualTo(v1, v2), nil
}

func (r *Runner) resolveNotEqualsEqualsBinaryExpression(expr *BinaryExpression, v1, v2 interface{}) (interface{}, error) {
	return !r.valueEqualTo(v1, v2), nil
}

func (r *Runner) valueEqualTo(v1, v2 interface{}) bool {
	if IsNull(v1) && IsNull(v2) || v1 == v2 {
		return true
	}
	if reflect.TypeOf(v1) == reflect.TypeOf(v2) {
		switch v1.(type) {
		case *decimal.Big:
			n1 := v1.(*decimal.Big)
			n2 := v2.(*decimal.Big)
			return n1.Cmp(n2) == 0
		case bool:
			n1 := v1.(bool)
			n2 := v2.(bool)
			return n1 == n2
		case string:
			s1 := v1.(string)
			s2 := v2.(string)
			return s1 == s2
		}
	}
	return false
}

func (r *Runner) resolveAmpersandAmpersandBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	if r.toBool(v1) {
		return v2, nil
	} else {
		return v1, nil
	}
}

func (r *Runner) resolveBarBarBinaryExpression(v1, v2 interface{}) (interface{}, error) {
	if !r.toBool(v1) {
		return v2, nil
	} else {
		return v1, nil
	}
}

func (r *Runner) resolveCommaBinaryExpression(_, v2 interface{}) (interface{}, error) {
	return v2, nil
}

func (r *Runner) resolveEqualBinaryExpression(ctx context.Context, left, right Expression) (interface{}, error) {
	if !Is[*Identifier](left) {
		return 0, errors.New("assignment expression left expression is not identifier")
	}
	identifierValue := left.(*Identifier).Value
	if !strings.HasPrefix(identifierValue, "$") {
		return 0, fmt.Errorf("assignment expression left identifier must start of '$' but %s", identifierValue)
	}
	v2, err := r.resolve(ctx, right)
	if err != nil {
		return nil, err
	}
	r.SetThisValue(identifierValue, v2)
	return v2, nil
}

func (r *Runner) resolveArrayLiteralExpression(ctx context.Context, expr *ArrayLiteralExpression) (interface{}, error) {
	var list []interface{}
	if expr.Elements != nil && expr.Elements.Len() > 0 {
		for i := 0; i < expr.Elements.Len(); i++ {
			item := expr.Elements.At(i)
			v1, err := r.resolve(ctx, item)
			if err != nil {
				return nil, err
			}
			list = append(list, v1)
		}
	}
	return list, nil
}

func (r *Runner) resolveParenthesizedExpression(ctx context.Context, expr *ParenthesizedExpression) (interface{}, error) {
	v, err := r.resolve(ctx, expr.Expression)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *Runner) resolveLiteralExpression(ctx context.Context, expr *LiteralExpression) (interface{}, error) {
	switch expr.Token {
	case SK_TrueKeyword:
		return true, nil
	case SK_FalseKeyword:
		return false, nil
	case SK_NullKeyword:
		return nil, nil
	case SK_ThisKeyword:
		return r.this, nil
	case SK_CtxKeyword:
		return ctx, nil
	case SK_NumberLiteral:
		r, ok := newDecimalBig().SetString(expr.Value)
		if !ok {
			return nil, fmt.Errorf("%s not number literal", expr.Value)
		}
		return r, nil
	case SK_StringLiteral:
		return r.resolveStringLiteralExpression(expr)
	}
	return nil, errors.New("unknown liternal expression")
}

func (r *Runner) resolveStringLiteralExpression(expr *LiteralExpression) (interface{}, error) {
	return expr.Value, nil
}

func (r *Runner) resolveConditionalExpression(ctx context.Context, expr *ConditionalExpression) (interface{}, error) {
	cond, err := r.resolve(ctx, expr.Condition)
	if err != nil {
		return nil, err
	}
	// if !IsBoolean(cond) {
	// 	return nil, errors.New("condition result value type not boolean")
	// }
	if r.toBool(cond) {
		v, err := r.resolve(ctx, expr.WhenTrue)
		if err != nil {
			return nil, err
		}
		return v, nil
	} else {
		v, err := r.resolve(ctx, expr.WhenFalse)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
}

func (r *Runner) resolveTypeofExpression(ctx context.Context, expr *TypeOfExpression) (interface{}, error) {
	value, err := r.resolve(ctx, expr.Expression)
	if err != nil {
		return nil, err
	}
	switch value.(type) {
	case bool:
		return "boolean", nil
	case string:
		return "string", nil
	case *decimal.Big:
		return "number", nil
	default:
		return "object", nil
	}
}

func convToString(v interface{}) string {
	switch n := v.(type) {
	case string:
		return n
	case *decimal.Big:
		return n.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func convToNumber(v interface{}) *decimal.Big {
	switch n := v.(type) {
	case *decimal.Big:
		return n
	case string:
		r, ok := newDecimalBig().SetString(n)
		if !ok {
			return newDecimalBig().SetNaN(true)
		}
		return r
	case bool:
		if n {
			return newDecimalBig().SetUint64(1)
		} else {
			return newDecimalBig().SetUint64(0)
		}
	default:
		if IsNull(v) {
			return newDecimalBig().SetUint64(0)
		} else {
			return newDecimalBig().SetNaN(true)
		}
	}
}

func (r *Runner) toBool(v interface{}) bool {
	switch n := v.(type) {
	case bool:
		return n
	case string:
		return len(n) > 0
	case *decimal.Big:
		return n.Cmp(newDecimalBig().SetUint64(0)) != 0 && !n.IsNaN(0)
	default:
		return !IsNull(v)
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

func funNow() (time.Time, error) {
	return time.Now(), nil
}

func funToDay() (time.Time, error) {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local), nil
}

func funDate(y, m, d int) (time.Time, error) {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local), nil
}

func funAddDate(date time.Time, y, m, d int) (time.Time, error) {
	return date.AddDate(y, m, d), nil
}

func funYear(date time.Time) (int, error) {
	return date.Year(), nil
}

func funMonth(date time.Time) (int, error) {
	return int(date.Month()), nil
}

func funDay(date time.Time) (int, error) {
	return date.Day(), nil
}

func funHour(date time.Time) (int, error) {
	return date.Hour(), nil
}

func funMinute(date time.Time) (int, error) {
	return date.Minute(), nil
}

func funSecond(date time.Time) (int, error) {
	return date.Second(), nil
}

func funMillSecond(date time.Time) (int64, error) {
	return date.UnixNano() / 1e6, nil
}

func funWeekDay(date time.Time) (int, error) {
	return int(date.Weekday()), nil
}

func funTimeFormat(date time.Time, layout string) (string, error) {
	return date.Format(layout), nil
}

func funUseTimezone(date time.Time, name string) (time.Time, error) {
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.Time{}, err
	}
	return date.In(location), nil
}

// FUNCTION MATH
func funAbs(v *decimal.Big) (*decimal.Big, error) {
	return newDecimalBig().Abs(v), nil
}

func funCeil(v *decimal.Big) (*decimal.Big, error) {
	result := newDecimalBig()
	decimal.Context64.Ceil(result, v)
	return result, nil
}

func funExp(v *decimal.Big) (*decimal.Big, error) {
	result := newDecimalBig()
	decimal.Context64.Exp(result, v)
	return result, nil
}

func funFloor(v *decimal.Big) (*decimal.Big, error) {
	result := newDecimalBig()
	decimal.Context64.Floor(result, v)
	return result, nil
}

func funLn(v *decimal.Big) (*decimal.Big, error) {
	result := newDecimalBig()
	decimal.Context64.Log(result, v)
	return result, nil
}

func funLog(v *decimal.Big) (*decimal.Big, error) {
	result := newDecimalBig()
	decimal.Context64.Log10(result, v)
	return result, nil
}

func funMax(nums ...*decimal.Big) (*decimal.Big, error) {
	if len(nums) == 0 {
		return nil, errors.New("please input numbers")
	}
	max := nums[0]
	for _, v := range nums {
		if v.Cmp(max) > 0 {
			max = v
		}
	}
	return max, nil
}

func funMin(nums ...*decimal.Big) (*decimal.Big, error) {
	if len(nums) == 0 {
		return nil, errors.New("please input numbers")
	}
	return decimal.Min(nums...), nil
}

func funRound(v *decimal.Big) (*decimal.Big, error) {
	return newDecimalBig().Round(0), nil
}

func funRoundBank(v *decimal.Big) (*decimal.Big, error) {
	// 将 v 的小数部分提取出来
	mv := newDecimalBig().Rem(v, decimal.New(1, 0))
	if mv.Cmp(decimal.New(5, -1)) <= 0 {
		return funCeil(v)
	} else {
		return funFloor(v)
	}
}

func funRoundCash(v, places *decimal.Big) (*decimal.Big, error) {
	mv := newDecimalBig().Rem(v, decimal.New(1, 0))
	if mv.Cmp(decimal.New(5, -2)) <= 0 {
		return funCeil(v)
	} else {
		return funFloor(v)
	}
}

func funSqrt(v *decimal.Big) (*decimal.Big, error) {
	result := newDecimalBig()
	decimal.Context64.Sqrt(result, v)
	return result, nil
}

func funFinite(v interface{}) (*decimal.Big, error) {
	if f, ok := v.(*decimal.Big); ok && f != nil && f.IsFinite() {
		return f, nil
	}
	return decimal.New(0, 0), nil
}

// FUNCTION STRING
func funStartWith(s string, substr string) (bool, error) {
	return strings.Index(s, substr) == 0, nil
}

func funEndWith(s string, substr string) (bool, error) {
	return strings.Index(s, substr) == len(s)-len(substr), nil
}

func funContains(s string, substr string) (bool, error) {
	return strings.Contains(s, substr), nil
}

func funFind(s string, substr string) (int, error) {
	return strings.Index(s, substr), nil
}

func funIncludes(list []string, item string) (bool, error) {
	for _, v := range list {
		if v == item {
			return true, nil
		}
	}
	return false, nil
}

func funLeft(v string, ld int) (string, error) {
	l := ld
	if l > len(v) {
		l = len(v)
	}
	return v[:l], nil
}

func funRight(v string, ld int) (string, error) {
	l := ld
	if l > len(v) {
		l = len(v)
	}
	return v[len(v)-l:], nil
}

func funLen(v string) (int, error) {
	return len(v), nil
}

func funLower(v string) (string, error) {
	return strings.ToLower(v), nil
}

func funUpper(v string) (string, error) {
	return strings.ToUpper(v), nil
}

func funLpad(s, ps string, l int) (string, error) {
	if len(s) > int(l) {
		return s[:int(l)], nil
	}
	return strings.Repeat(ps, l-len(s)) + s, nil
}

func funRpad(s, ps string, l int) (string, error) {
	if len(s) > int(l) {
		return s[:int(l)], nil
	}
	return s + strings.Repeat(ps, l-len(s)), nil
}

func funMid(s string, start, end int) (string, error) {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	return s[start:end], nil
}

func funReplace(s, old, new string) (string, error) {
	return strings.ReplaceAll(s, old, new), nil
}

func funTrim(s string) (string, error) {
	return strings.TrimSpace(s), nil
}

func funRegexp(s string, reg string) (bool, error) {
	return regexp.MustCompile(reg).Match([]byte(s)), nil
}

func funMapToArr(m []map[string]any, key string) ([]any, error) {
	var result []any
	for _, v := range m {
		result = append(result, v[key])
	}
	return result, nil
}

func funJoin(arr []string, join string) (string, error) {
	return strings.Join(arr, join), nil
}

// CONV
func funToString(v interface{}) (string, error) {
	return convToString(v), nil
}

func funToInt(v interface{}) (*decimal.Big, error) {
	n := convToNumber(v)
	iv, _ := n.Int64()
	return newDecimalBig().SetFloat64(float64(iv)), nil
}

func funToFloat(v interface{}) (*decimal.Big, error) {
	return convToNumber(v), nil
}
