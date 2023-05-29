package compiler

import "fmt"

func IsNumber(v interface{}) bool {
	switch v.(type) {
	case int, int32, int64, float32, float64:
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

func ToInt(v interface{}) (int32, error) {
	switch n := v.(type) {
	case int:
		return int32(n), nil
	case int32:
		return int32(n), nil
	case int64:
		return int32(n), nil
	case float32:
		return int32(n), nil
	case float64:
		return int32(n), nil
	}
	return 0, fmt.Errorf("ToInt not support type %T", v)
}

func ToLong(v interface{}) (int64, error) {
	switch n := v.(type) {
	case int32:
		return int64(n), nil
	case int64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	case float64:
		return int64(n), nil
	}
	return 0, fmt.Errorf("ToLong not support type %T", v)
}

func ToFloat(v interface{}) (float32, error) {
	switch n := v.(type) {
	case int32:
		return float32(n), nil
	case int64:
		return float32(n), nil
	case float32:
		return float32(n), nil
	case float64:
		return float32(n), nil
	}
	return 0, fmt.Errorf("ToFloat not support type %T", v)
}

func ToDouble(v interface{}) (float64, error) {
	switch n := v.(type) {
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return float64(n), nil
	}
	return 0, fmt.Errorf("ToDouble not support type %T", v)
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
