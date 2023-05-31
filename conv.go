package formula

import (
	"fmt"

	"github.com/shopspring/decimal"
)

func IsNumber(v interface{}) bool {
	switch v.(type) {
	case decimal.Decimal:
		return true
	default:
		return false
	}
}

func IsIntOrLong(v interface{}) bool {
	switch v.(type) {
	case int32, int64:
		return true
	default:
		return false
	}
}

func IsInt(v interface{}) bool {
	_, ok := v.(int32)
	return ok
}

func IsLong(v interface{}) bool {
	_, ok := v.(int64)
	return ok
}

func IsBoolean(v interface{}) bool {
	_, ok := v.(bool)
	return ok
}

func IsString(v interface{}) bool {
	_, ok := v.(string)
	return ok
}

func ToNumber(v interface{}) (decimal.Decimal, error) {
	switch n := v.(type) {
	case int:
		return decimal.NewFromInt(int64(n)), nil
	case int32:
		return decimal.NewFromInt(int64(n)), nil
	case int64:
		return decimal.NewFromInt(int64(n)), nil
	case float32:
		return decimal.NewFromInt(int64(n)), nil
	case float64:
		return decimal.NewFromInt(int64(n)), nil
	default:
		return decimal.Decimal{}, fmt.Errorf("ToNumber not support type %T", v)
	}
}

func FormatValue(v interface{}) (interface{}, error) {
	switch n := v.(type) {
	case int:
		return decimal.NewFromInt(int64(n)), nil
	case int32:
		return decimal.NewFromInt(int64(n)), nil
	case int64:
		return decimal.NewFromInt(int64(n)), nil
	case float32:
		return decimal.NewFromInt(int64(n)), nil
	case float64:
		return decimal.NewFromInt(int64(n)), nil
	case string:
		return n, nil
	case bool:
		return n, nil
	case nil:
		return nil, nil
	default:
		return decimal.Decimal{}, fmt.Errorf("FormatValue not support type %T", v)
	}
}

func ToInt(v interface{}) (int, error) {
	switch n := v.(type) {
	case decimal.Decimal:
		return int(n.IntPart()), nil
	default:
		return 0, fmt.Errorf("ToInt not support type %T", v)
	}
}

func ToInt32(v interface{}) (int32, error) {
	switch n := v.(type) {
	case decimal.Decimal:
		return int32(n.IntPart()), nil
	default:
		return 0, fmt.Errorf("ToInt not support type %T", v)
	}
}

func ToInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case decimal.Decimal:
		return int64(n.IntPart()), nil
	default:
		return 0, fmt.Errorf("ToInt not support type %T", v)
	}
}

func ToFloat32(v interface{}) (float32, error) {
	switch n := v.(type) {
	case decimal.Decimal:
		return float32(n.InexactFloat64()), nil
	default:
		return 0, fmt.Errorf("ToInt not support type %T", v)
	}
}

func ToFloat64(v interface{}) (float64, error) {
	switch n := v.(type) {
	case decimal.Decimal:
		return float64(n.InexactFloat64()), nil
	default:
		return 0, fmt.Errorf("ToInt not support type %T", v)
	}
}

func ToString(v interface{}) (string, error) {
	switch n := v.(type) {
	case string:
		return n, nil
	}
	return "", fmt.Errorf("ToString not support type %T", v)
}

func ToBool(v interface{}) (bool, error) {
	switch n := v.(type) {
	case bool:
		return n, nil
	}
	return false, fmt.Errorf("ToBool not support type %T", v)
}
