package formula

import (
	"errors"
	"strings"
)

func ResolveReferenceFields(source *SourceCode) ([]string, error) {
	resolve := referenceResovle{}
	err := resolve.resolve(source.Expression)
	if err != nil {
		return nil, err
	}
	return stringsUniq(resolve.fields), nil
}

func ResolveReferenceFieldsNotLocal(source *SourceCode) ([]string, error) {
	fields, err := ResolveReferenceFields(source)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, field := range fields {
		if !strings.HasPrefix(field, "$") {
			result = append(result, field)
		}
	}
	return result, nil
}

type referenceResovle struct {
	fields []string
}

func (r *referenceResovle) resolve(node Node) error {
	switch n := node.(type) {
	case *Identifier:
		return r.resolveIdentifier(n)
	case *PrefixUnaryExpression:
		return r.resolvePrefixUnaryExpression(n)
	case *BinaryExpression:
		return r.resolveBinaryExpression(n)
	case *ArrayLiteralExpression:
		return r.resolveArrayLiteralExpression(n)
	case *ParenthesizedExpression:
		return r.resolveParenthesizedExpression(n)
	case *LiteralExpression:
		return r.resolveLiteralExpression(n)
	case *SelectorExpression:
		return r.resolveSelectorExpression(n)
	case *CallExpression:
		return r.resolveCallExpression(n)
	case *ConditionalExpression:
		return r.resolveConditionalExpression(n)
	case *TypeOfExpression:
		return r.resolveTypeofExpression(n)
	default:
		return errors.New("unknown expression type")
	}
}

func (r *referenceResovle) resolveIdentifier(v *Identifier) error {
	r.fields = append(r.fields, v.Value)
	return nil
}

func (r *referenceResovle) resolvePrefixUnaryExpression(v *PrefixUnaryExpression) error {
	return r.resolve(v.Operand)
}

func (r *referenceResovle) resolveBinaryExpression(v *BinaryExpression) error {
	err := r.resolve(v.Left)
	if err != nil {
		return err
	}
	err = r.resolve(v.Right)
	if err != nil {
		return err
	}
	return nil
}

func (r *referenceResovle) resolveArrayLiteralExpression(v *ArrayLiteralExpression) error {
	if v.Elements != nil && v.Elements.Len() > 0 {
		for i := 0; i < v.Elements.Len(); i++ {
			err := r.resolve(v.Elements.At(i))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *referenceResovle) resolveParenthesizedExpression(v *ParenthesizedExpression) error {
	return r.resolve(v.Expression)
}

func (r *referenceResovle) resolveLiteralExpression(v Expression) error {
	return nil
}

func (r *referenceResovle) resolveSelectorExpression(v *SelectorExpression) error {
	names, err := resolveSelecotrNames(v)
	if err != nil {
		return err
	}
	r.fields = append(r.fields, strings.Join(names, "."))
	return nil
}

func (r *referenceResovle) resolveCallExpression(v *CallExpression) error {
	if v.Arguments != nil && v.Arguments.Len() > 0 {
		for i := 0; i < v.Arguments.Len(); i++ {
			err := r.resolve(v.Arguments.At(i))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *referenceResovle) resolveConditionalExpression(v *ConditionalExpression) error {
	err := r.resolve(v.Condition)
	if err != nil {
		return err
	}
	err = r.resolve(v.WhenTrue)
	if err != nil {
		return err
	}
	err = r.resolve(v.WhenFalse)
	if err != nil {
		return err
	}
	return nil
}

func (r *referenceResovle) resolveTypeofExpression(v *TypeOfExpression) error {
	err := r.resolve(v.Expression)
	if err != nil {
		return err
	}
	return nil
}
