package compiler

import (
	"testing"
)

func TestCompareExpression(t *testing.T) {
	data := []string{
		"a == b",
		"a > b",
		"a >= b",
		"a < b",
		"a <= b",
	}

	for i, str := range data {
		_, err := ParseSourceCode([]byte(str))
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			return
		}
	}
}

func TestSelectorExpression(t *testing.T) {
	data := []string{
		"a.b",
		"a.b.c",
	}

	for i, str := range data {
		_, err := ParseSourceCode([]byte(str))
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			return
		}
	}
}

func TestCallExpression(t *testing.T) {
	data := []string{
		"a()",
		"a(1, 2, 3)",
		"a.b()",
	}

	for i, str := range data {
		_, err := ParseSourceCode([]byte(str))
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			return
		}
	}
}

func TestArrayLiteralExpression(t *testing.T) {
	data := []string{
		"[1, 2, 3]",
		"[1]",
		"[true, 1, '2']",
	}

	for i, str := range data {
		_, err := ParseSourceCode([]byte(str))
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			return
		}
	}
}

func TestParenthesizedExpression(t *testing.T) {
	data := []string{
		"(1)",
		"(1 == 2)",
		"(1 + 3 == 4)",
	}

	for i, str := range data {
		_, err := ParseSourceCode([]byte(str))
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			return
		}
	}
}

func TestLiteralExpression(t *testing.T) {
	data := []string{
		"1",
		"1f",
		"1d",
		"2.0",
		"2.0f",
		"2.0d",
		"100L",
		"10e5",
		"true",
		"false",
		"null",
	}

	for i, str := range data {
		_, err := ParseSourceCode([]byte(str))
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			return
		}
	}
}

func TestPrefixUnaryExpression(t *testing.T) {
	data := []string{
		"+1",
		"-1",
		"!1",
		"~1",
	}

	for i, str := range data {
		_, err := ParseSourceCode([]byte(str))
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			return
		}
	}
}

func TestConditionalExpression(t *testing.T) {
	data := []string{
		"1 > 2 ? true : false",
	}

	for i, str := range data {
		_, err := ParseSourceCode([]byte(str))
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			return
		}
	}
}
