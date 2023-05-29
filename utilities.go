package formula

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func Is[T any](n any) bool {
	_, ok := n.(T)
	return ok
}

func IsNull(i interface{}) bool {
	if i == nil {
		return true
	}
	vi := reflect.ValueOf(i)
	if vi.Kind() == reflect.Ptr {
		return vi.IsNil()
	}
	return false
}

func assertMsg(expr bool, msg string) {
	if !expr {
		panic(msg)
	}
}

func CreateFileDiagnostic(file *SourceCode, start int, length int, msg *DiagnosticMessage, args ...interface{}) *Diagnostic {
	if start < 0 {
		panic("start must be non-negative, is " + strconv.Itoa(start))
	}
	if length < 0 {
		panic("length must be non-negative, is " + strconv.Itoa(length))
	}

	var text = msg.Message
	if len(args) > 0 {
		text = formatStringFromArgs(text, args...)
	}

	return &Diagnostic{
		File:        file,
		Start:       start,
		Length:      length,
		MessageText: text,
		Category:    msg.Category,
		Code:        msg.Code,
	}
}

func formatStringFromArgs(text string, args ...interface{}) string {
	for i, arg := range args {
		text = strings.ReplaceAll(text, fmt.Sprintf("{%d}", i), toString(arg))
	}
	return text
}

func toString(value interface{}) string {
	var key string
	if value == nil {
		return key
	}

	switch f := value.(type) {
	case float64:
		key = strconv.FormatFloat(f, 'f', -1, 64)
	case float32:
		key = strconv.FormatFloat(float64(f), 'f', -1, 64)
	case int:
		key = strconv.Itoa(f)
	case uint:
		key = strconv.Itoa(int(f))
	case int8:
		key = strconv.Itoa(int(f))
	case uint8:
		key = strconv.Itoa(int(f))
	case int16:
		key = strconv.Itoa(int(f))
	case uint16:
		key = strconv.Itoa(int(f))
	case int32:
		key = strconv.Itoa(int(f))
	case uint32:
		key = strconv.Itoa(int(f))
	case int64:
		key = strconv.FormatInt(f, 10)
	case uint64:
		key = strconv.FormatUint(f, 10)
	case string:
		key = value.(string)
	case []byte:
		key = string(value.([]byte))
	default:
		newValue, _ := json.Marshal(value)
		key = string(newValue)
	}

	return key
}

func FormatDiagnostic(source *SourceCode, diagnostic *Diagnostic) string {
	var loc = GetFileLineAndCharacterFromPosition(source, diagnostic.Start)
	var category = strings.ToLower(diagnostic.Category.ToString())
	return fmt.Sprintf("pos(%d, %d) %s(%d) %s", loc.Line, loc.Column, category, diagnostic.Code, diagnostic.MessageText)
}
