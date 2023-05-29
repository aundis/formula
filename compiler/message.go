package compiler

var (
	M_Invalid_character = &DiagnosticMessage{
		Code:     1127,
		Category: Error,
		Message:  "invalid character",
	}

	M_Digit_expected = &DiagnosticMessage{
		Code:     1124,
		Category: Error,
		Message:  "digit expected",
	}

	M_0_expected = &DiagnosticMessage{
		Code:     1005,
		Category: Error,
		Message:  "{0} expected",
	}

	M_Identifier_expected = &DiagnosticMessage{
		Code:     1003,
		Category: Error,
		Message:  "identifier expected",
	}

	M_Hexadecimal_digit_expected = &DiagnosticMessage{
		Code:     1125,
		Category: Error,
		Message:  "hexadecimal digit expected",
	}

	M_Expression_expected = &DiagnosticMessage{
		Code:     1109,
		Category: Error,
		Message:  "expression excepted",
	}

	M_Argument_expression_expected = &DiagnosticMessage{
		Code:     1135,
		Category: Error,
		Message:  "argument expression expected",
	}

	M_Expression_or_comma_expected = &DiagnosticMessage{
		Code:     1137,
		Category: Error,
		Message:  "expression or comma expected",
	}

	M_Unexpected_end_of_text = &DiagnosticMessage{
		Code:     1126,
		Category: Error,
		Message:  "unexpected end of text",
	}

	M_Unterminated_string_literal = &DiagnosticMessage{
		Code:     1002,
		Category: Error,
		Message:  "unterminated string literal",
	}

	M_Multiple_consecutive_numeric_separators_are_not_permitted = &DiagnosticMessage{
		Code:     1301,
		Category: Error,
		Message:  "multiple cconsecutive numeric separators are not permitted",
	}

	M_Numeric_separators_are_not_allowed_here = &DiagnosticMessage{
		Code:     1302,
		Category: Error,
		Message:  "numeric separators are not allowed here",
	}

	M_An_identifier_or_keyword_cannot_immediately_follow_a_numeric_literal = &DiagnosticMessage{
		Code:     1302,
		Category: Error,
		Message:  "an identifier or keyword cannot immediately follow a numeric literal",
	}

	M_Trailing_comma_not_allowed = &DiagnosticMessage{
		Code:     1009,
		Category: Error,
		Message:  "trailing comma not allowed",
	}
)
