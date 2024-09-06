package formula

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ErrorHandler func(msg *DiagnosticMessage, pos int, length int)

type TokenFlags int

const (
	TF_None               TokenFlags = 0
	TF_PrecedingLineBreak TokenFlags = 1 << iota
	TF_Scientific                    // e.g. `10e2`
	TF_Decimal                       // e.g  `0.123`
	TF_Octal                         // e.g. `0777`
	TF_HexSpecifier                  // e.g. `0x00000000`
	TF_BinarySpecifier               // e.g. `0b0110010000000000`
	TF_OctalSpecifier                // e.g. `0o777`
	TF_ContainsSeparator             // e.g. `0b1100_0101`
	TF_UnicodeEscape                 // e.g. `\u0000`
)

type Scanner struct {
	text []byte
	// Current position (end position of text of current token)
	pos int
	// end of text
	end int
	// Start position of whitespace before current token
	startPos int
	// Start position of text of current token
	tokenPos int
	// Token info
	token      SyntaxKind
	tokenValue string
	tokenFlags TokenFlags
	// Report error
	onError ErrorHandler
}

func CreateScanner(text []byte, onError ErrorHandler) *Scanner {
	var scanner = new(Scanner)
	scanner.onError = onError
	scanner.SetText(text)
	return scanner
}

func (s *Scanner) SetOnError(fun ErrorHandler) {
	s.onError = fun
}

func (s *Scanner) error(msg *DiagnosticMessage) {
	if s.onError != nil {
		s.onError(msg, -1, 0)
	}
}

func (s *Scanner) errorAtPos(msg *DiagnosticMessage, pos int, length int) {
	if s.onError != nil {
		s.onError(msg, pos, length)
	}
}

func (s *Scanner) isIdentifierStart(Ch rune) bool {
	return IsIdentifierStart(Ch)
}

func (s *Scanner) isIdentifierPart(Ch rune) bool {
	return IsIdentifierPart(Ch)
}

func (s *Scanner) scanNumberFragment() string {
	var start = s.pos
	var allowSeparator = false
	var isPreviousTokenSeparator = false
	var underlineStart int
	var result bytes.Buffer

	for s.pos < s.end {
		ch, size := utf8.DecodeRune(s.text[s.pos:])
		if ch == '_' {
			s.tokenFlags |= TF_ContainsSeparator
			if allowSeparator {
				allowSeparator = false
				isPreviousTokenSeparator = true
				result.Write(s.text[start:s.pos])
			} else if isPreviousTokenSeparator {
				s.errorAtPos(M_Multiple_consecutive_numeric_separators_are_not_permitted, s.pos, 1)
			} else {
				s.errorAtPos(M_Numeric_separators_are_not_allowed_here, s.pos, 1)
			}

			start = s.pos
			underlineStart = s.pos
			s.pos += size
			continue
		}

		if IsDigit(ch) {
			allowSeparator = true
			isPreviousTokenSeparator = false
			s.pos += size
			continue
		}

		break
	}

	if isPreviousTokenSeparator {
		s.errorAtPos(M_Numeric_separators_are_not_allowed_here, underlineStart, 1)
	}

	result.Write(s.text[start:s.pos])
	return result.String()
}

func (s *Scanner) scanNumber() (SyntaxKind, string) {
	var start = s.pos
	var mainFragment = s.scanNumberFragment()
	var decimalFragment string
	var scientificFragment string
	if tar := s.peekEqual(0, '.'); tar >= 0 {
		s.tokenFlags |= TF_Decimal
		s.pos = tar
		decimalFragment = s.scanNumberFragment()
	}

	var end = s.pos
	if tar := s.peekCheck(0, func(ch rune) bool { return ch == 'e' || ch == 'E' }); tar >= 0 {
		s.pos = tar
		s.tokenFlags |= TF_Scientific
		if tar := s.peekCheck(0, func(ch rune) bool { return ch == '+' || ch == '-' }); tar >= 0 {
			s.pos = tar
		}

		var preNumericPart = s.pos
		var finalFragment = s.scanNumberFragment()
		if len(finalFragment) == 0 {
			s.error(M_Digit_expected)
		} else {
			scientificFragment = string(s.text[end:preNumericPart]) + finalFragment
			end = s.pos
		}
	}

	var result string
	if s.tokenFlags&TF_ContainsSeparator != 0 {
		result = mainFragment
		if len(decimalFragment) > 0 {
			result += "." + decimalFragment
		}
		if len(scientificFragment) > 0 {
			result += scientificFragment
		}
	} else {
		result = string(s.text[start:end]) // No need to use all the fragments; no _ removal needed
	}

	s.tokenValue = result
	// var kind = s.checkNumberSuffix()
	s.checkForIdentifierStartAfterNumericLiteral()
	// return kind, s.tokenValue
	return SK_NumberLiteral, s.tokenValue
}

func (s *Scanner) checkForIdentifierStartAfterNumericLiteral() {
	ch, _ := s.peek(s.pos)
	if !s.isIdentifierStart(ch) {
		return
	}

	var identifierStart = s.pos
	var length = len(s.scanIdentifierParts())

	s.errorAtPos(M_An_identifier_or_keyword_cannot_immediately_follow_a_numeric_literal, identifierStart, length)
	s.pos = identifierStart
}

// Scans the given number of hexadecimal digits in the text,
// returning -1 if the given number is unavailable.
func (s *Scanner) scanExactNumberOfHexDigits(count int, canHaveSeparators bool) int {
	var valueString = s.scanHexDigits(count, false, canHaveSeparators)
	if len(valueString) > 0 {
		value, err := strconv.ParseInt(valueString, 16, 64)
		if err != nil {
			panic(fmt.Sprintf("parse int error, value is (%s).", valueString))
		}
		return int(value)
	}
	return -1
}

// Scans as many hexadecimal digits as are available in the text,
// returning "" if the given number of digits was unavailable.
func (s *Scanner) scanMinimumNumberOfHexDigits(count int, canHaveSeparators bool) string {
	return s.scanHexDigits(count, true, canHaveSeparators)
}

func (s *Scanner) scanHexDigits(count int, scanAsManyAsPossible bool, canHaveSeparators bool) string {
	var valueChars []rune
	var allowSeparator = false
	var underlineStart int
	var isPreviousTokenSeparator = false
	for (len(valueChars) < count || scanAsManyAsPossible) && s.pos < s.end {
		ch, size := utf8.DecodeRune(s.text[s.pos:])
		if canHaveSeparators && ch == '_' {
			s.tokenFlags |= TF_ContainsSeparator
			if allowSeparator {
				allowSeparator = false
				isPreviousTokenSeparator = true
			} else if isPreviousTokenSeparator {
				s.errorAtPos(M_Multiple_consecutive_numeric_separators_are_not_permitted, s.pos, 1)
			} else {
				s.errorAtPos(M_Numeric_separators_are_not_allowed_here, s.pos, 1)
			}
			underlineStart = s.pos
			s.pos += size
			continue
		}
		allowSeparator = canHaveSeparators
		if ch >= 'A' && ch <= 'F' {
			ch += 'a' - 'A'
		} else if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z') {
			break
		}

		valueChars = append(valueChars, ch)
		s.pos += size
		isPreviousTokenSeparator = false
	}

	if isPreviousTokenSeparator {
		s.errorAtPos(M_Numeric_separators_are_not_allowed_here, underlineStart, 1)
	}
	return string(valueChars)
}

func (s *Scanner) scanString() string {
	ch, size := utf8.DecodeRune(s.text[s.pos:])
	var quote = ch
	s.pos += size

	var contents strings.Builder
	var start = s.pos
	for {
		if s.pos >= s.end {
			contents.Write(s.text[start:s.pos])
			s.error(M_Unexpected_end_of_text)
			break
		}

		ch, size = utf8.DecodeRune(s.text[s.pos:])
		if ch == quote {
			contents.Write(s.text[start:s.pos])
			s.pos += size
			break
		}
		if ch == '\\' {
			contents.Write(s.text[start:s.pos])
			contents.WriteString(s.scanEscapeSequence())
			start = s.pos
			continue
		}
		if IsLineBreak(ch) {
			contents.Write(s.text[start:s.pos])
			s.error(M_Unterminated_string_literal)
			break
		}
		s.pos += size
	}
	return contents.String()
}

func (s *Scanner) scanEscapeSequence() string {
	s.pos++
	if s.pos >= s.end {
		s.error(M_Unexpected_end_of_text)
		return ""
	}

	ch, size := utf8.DecodeRune(s.text[s.pos:])
	s.pos += size
	switch ch {
	case '0':
		return "\000"
	case 'b':
		return "\b"
	case 't':
		return "\t"
	case 'n':
		return "\n"
	case 'v':
		return "\v"
	case 'f':
		return "\f"
	case 'r':
		return "\r"
	case '\'':
		return "'"
	case '"':
		return "\""
	case 'u': // '\uDDDD'
		s.tokenFlags |= TF_UnicodeEscape
		return s.scanHexadecimalEscape(4)
	case 'x': // '\xDD'
		return s.scanHexadecimalEscape(2)
	case '\r':
		if tar := s.peekEqual(1, '\n'); tar >= 0 {
			s.pos = tar
		}
		fallthrough
	case '\n', Uni_LineSeparator, Uni_ParagraphSeparator:
		return ""
	default:
		return string(ch)
	}
}

func (s *Scanner) scanHexadecimalEscape(numDigits int) string {
	var escapedValue = s.scanExactNumberOfHexDigits(numDigits, false)

	if escapedValue >= 0 {
		return strconv.Itoa(escapedValue)
	} else {
		s.error(M_Hexadecimal_digit_expected)
		return ""
	}
}

// func (s *Scanner) checkNumberSuffix() SyntaxKind {
// 	ch, size := utf8.DecodeRune(s.text[s.pos:])
// 	switch ch {
// 	case 'f', 'F':
// 		s.pos += size
// 		return SK_FloatLiteral
// 	case 'd', 'D':
// 		s.pos += size
// 		return SK_DoubleLiteral
// 	}
// 	if s.tokenFlags&TF_Scientific != 0 {
// 		return SK_DoubleLiteral
// 	}
// 	if s.tokenFlags&TF_Decimal != 0 {
// 		return SK_FloatLiteral
// 	}
// 	switch ch {
// 	case 'l', 'L':
// 		s.pos += size
// 		return SK_LongLiteral
// 	}
// 	return SK_IntLiteral
// }

// Current character is known to be a backslash. Check for Unicode escape of the form '\uXXXX'
// and return code point value if valid Unicode escape is found. Otherwise return -1.
func (s *Scanner) peekUnicodeEscape() rune {
	if s.pos+5 < s.end {
		if tar := s.peekEqual(1, 'u'); tar >= 0 {
			var start = s.pos
			var value = s.scanExactNumberOfHexDigits(4, true)
			s.pos = start
			return rune(value)
		}
	}
	return -1
}

func (s *Scanner) scanIdentifierParts() string {
	var result = ""
	var start = s.pos
	for s.pos < s.end {
		ch, size := utf8.DecodeRune(s.text[s.pos:])
		if s.isIdentifierPart(ch) {
			s.pos += size
		} else if ch == '\\' {
			var ch = s.peekUnicodeEscape()
			if !(ch >= 0 && s.isIdentifierPart(ch)) {
				break
			}

			result += string(s.text[start:s.startPos])
			result += string(ch)
			// Valid Unicode escape is always six characters
			s.pos += 6
			start = s.pos
		} else {
			break
		}
	}
	result += string(s.text[start:s.pos])
	return result
}

func (s *Scanner) getIdentifierToken() SyntaxKind {
	var tok = KeywordFromString(s.tokenValue)
	if tok != SK_Unknown {
		s.token = tok
		return s.token
	}
	s.token = SK_Identifier
	return s.token
}

// The count of calls of peekCheck is 6 times that of PeekEqual
func (s *Scanner) peekCheck(n int, f func(ch rune) bool) int {
	if s.pos >= s.end {
		return -1
	}
	var start = s.pos
	for start < s.end {
		ch, size := utf8.DecodeRune(s.text[start:])
		start += size
		n--
		if n < 0 {
			if f(ch) {
				return start
			}
			break
		}
	}
	return -1
}

func (s *Scanner) peek(pos int) (rune, int) {
	if pos < len(s.text) {
		ch, size := utf8.DecodeRune(s.text[pos:])
		return ch, size
	}

	return 0, 0
}

func (s *Scanner) peekEqual(n int, ch rune) int {
	if s.pos >= s.end {
		return -1
	}
	var start = s.pos
	for start < s.end {
		cur, size := utf8.DecodeRune(s.text[start:])
		start += size
		n--
		if n < 0 {
			if cur == ch {
				return start
			}
			break
		}
	}
	return -1
}

func (s *Scanner) Scan() SyntaxKind {
	s.startPos = s.pos
	s.tokenFlags = TF_None
	for {
		s.tokenPos = s.pos
		if s.pos >= s.end {
			s.token = SK_EndOfFile
			return s.token
		}

		ch, size := utf8.DecodeRune(s.text[s.pos:])
		switch ch {
		case '\n', '\r':
			s.tokenFlags |= TF_PrecedingLineBreak
			s.pos += size
			continue
		case '\t', '\v', '\f', ' ':
			s.pos += size
			continue
		case '!':
			if tar := s.peekEqual(1, '='); tar >= 0 {
				if tar := s.peekEqual(2, '='); tar >= 0 {
					s.pos = tar
					s.token = SK_ExclamationEqualsEquals
					return s.token
				}
				s.pos = tar
				s.token = SK_ExclamationEquals
				return s.token
			}
			if tar := s.peekEqual(1, '!'); tar >= 0 {
				s.pos = tar
				s.token = SK_ExclamationExclamation
				return s.token
			}
			if tar := s.peekEqual(1, '.'); tar >= 0 {
				s.pos = tar
				s.token = SK_ExclamationDot
				return s.token
			}
			s.pos += size
			s.token = SK_Exclamation
			return s.token
		case '"':
			s.tokenValue = s.scanString()
			s.token = SK_StringLiteral
			return s.token
		case '\'':
			s.tokenValue = s.scanString()
			s.token = SK_StringLiteral
			return s.token
		case '&':
			if tar := s.peekEqual(1, '&'); tar >= 0 {
				s.pos = tar
				s.token = SK_AmpersandAmpersand
				return s.token
			}
			s.pos += size
			s.token = SK_Ampersand
			return s.token
		case '(':
			s.pos += size
			s.token = SK_OpenParen
			return s.token
		case ')':
			s.pos += size
			s.token = SK_CloseParen
			return s.token
		case '%':
			s.pos += 1
			s.token = SK_Percent
			return s.token
		case '*':
			s.pos += 1
			s.token = SK_Asterisk
			return s.token
		case '+':
			s.pos += size
			s.token = SK_Plus
			return s.token
		case ',':
			s.pos += size
			s.token = SK_Comma
			return s.token
		case '-':
			s.pos += size
			s.token = SK_Minus
			return s.token
		case '.':
			if s.peekCheck(1, IsDigit) > 0 {
				s.token, s.tokenValue = s.scanNumber()
				return s.token
			}
			if tar := s.peekEqual(1, '.'); tar >= 0 {
				if tar := s.peekEqual(2, '.'); tar >= 0 {
					s.pos = tar
					s.token = SK_DotDotDot
					return s.token
				}
			}
			s.pos += size
			s.token = SK_Dot
			return s.token
		case '/':
			s.pos += size
			s.token = SK_Slash
			return s.token
		case '0':
			if s.pos+2 < s.end {
				if tar := s.peekCheck(1, func(ch rune) bool { return ch == 'x' || ch == 'X' }); tar >= 0 {
					s.pos = tar
					s.tokenValue = s.scanMinimumNumberOfHexDigits(1, false)
					if len(s.tokenValue) == 0 {
						s.error(M_Hexadecimal_digit_expected)
						s.tokenValue = "0"
					}
					s.tokenValue = "0x" + s.tokenValue
					s.tokenFlags |= TF_HexSpecifier
					// s.token = s.checkNumberSuffix()
					// return s.token
					return SK_NumberLiteral
				}
			}
			// This fall-through is a deviation from the EcmaScript grammar. The grammar says that a leading zero
			// can only be followed by an octal digit, a dot, or the end of the number literal. However, we are being
			// permissive and allowing decimal digits of the form 08* and 09* (which many browsers also do).
			fallthrough
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			s.token, s.tokenValue = s.scanNumber()
			return s.token
		case ':':
			s.pos += size
			s.token = SK_Colon
			return s.token
		case '<':
			if tar := s.peekEqual(1, '='); tar >= 0 {
				s.pos = tar
				s.token = SK_LessThanEquals
				return s.token
			}
			s.pos += size
			s.token = SK_LessThan
			return s.token
		case '=':
			if tar := s.peekEqual(1, '='); tar >= 0 {
				if tar := s.peekEqual(2, '='); tar >= 0 {
					s.pos = tar
					s.token = SK_EqualsEqualsEquals
					return s.token
				}
				s.pos = tar
				s.token = SK_EqualsEquals
				return s.token
			}
			s.pos += size
			s.token = SK_Equals
			return s.token
		case '>':
			if tar := s.peekEqual(1, '='); tar >= 0 {
				s.pos = tar
				s.token = SK_GreaterThanEquals
				return s.token
			}
			s.pos += size
			s.token = SK_GreaterThan
			return s.token
		case '?':
			if tar := s.peekEqual(1, '?'); tar >= 0 {
				s.pos = tar
				s.token = SK_QuestionQuestion
				return s.token
			}
			s.pos += size
			s.token = SK_Question
			return s.token
		case '[':
			s.pos += size
			s.token = SK_OpenBracket
			return s.token
		case ']':
			s.pos += size
			s.token = SK_CloseBracket
			return s.token
		case '^':
			s.pos += size
			s.token = SK_Caret
			return s.token
		case '|':
			if tar := s.peekEqual(1, '|'); tar >= 0 {
				s.pos = tar
				s.token = SK_BarBar
				return s.token
			}
			s.pos += size
			s.token = SK_Bar
			return s.token
		case '~':
			s.pos += size
			s.token = SK_Tilde
			return s.token
		default:
			if s.isIdentifierStart(ch) {
				s.pos += size
				for tar := s.pos; tar >= 0; tar = s.peekCheck(0, s.isIdentifierPart) {
					s.pos = tar
				}
				s.tokenValue = string(s.text[s.tokenPos:s.pos])
				if s.peekEqual(0, '\\') > 0 {
					s.tokenValue += s.scanIdentifierParts()
				}
				s.token = s.getIdentifierToken()
				return s.token
			} else if IsWhiteSpace(ch) {
				s.pos += size
				continue
			} else if IsLineBreak(ch) {
				s.tokenFlags |= TF_PrecedingLineBreak
				s.pos += size
				continue
			}
			s.error(M_Invalid_character)
			s.pos += size
			s.token = SK_Unknown
			return s.token
		}
	}
	return SK_Unknown
}

func (s *Scanner) SetText(newText []byte) {
	// full start and length
	s.pos = 0
	s.end = len(newText)

	s.text = newText
	s.SetTextPos(s.pos)
}

func (s *Scanner) SetTextPos(textPos int) {
	if textPos < 0 {
		panic("textPos < 0")
	}
	s.pos = textPos
	s.startPos = textPos
	s.tokenPos = textPos
	s.token = SK_Unknown
	s.tokenFlags = TF_None
}

func (s *Scanner) GetStartPos() int {
	return s.startPos
}

func (s *Scanner) GetToken() SyntaxKind {
	return s.token
}

func (s *Scanner) GetTextPos() int {
	return s.pos
}

func (s *Scanner) GetTokenPos() int {
	return s.tokenPos
}

func (s *Scanner) GetTokenText() string {
	return string(s.text[s.tokenPos:s.pos])
}

func (s *Scanner) GetTokenValue() string {
	return s.tokenValue
}

func (s *Scanner) HasPrecedingLineBreak() bool {
	return s.tokenFlags&TF_PrecedingLineBreak != 0
}

func (s *Scanner) isIdentifier() bool {
	return s.token == SK_Identifier
}

func scannerSpeculationHelper[T any](s *Scanner, callback func() T, isLookahead bool) T {
	var token = s.token
	var pos = s.pos
	var tokenValue = s.tokenValue
	var startPos = s.startPos
	var tokenPos = s.tokenPos
	var tokenFlags = s.tokenFlags
	var result = callback()
	// If our callback returned something 'falsy' or we're just looking ahead,
	// then unconditionally restore us to where we were.
	if IsNull(result) || isLookahead {
		s.token = token
		s.pos = pos
		s.tokenValue = tokenValue
		s.startPos = startPos
		s.tokenPos = tokenPos
		s.tokenFlags = tokenFlags
	}
	return result
}

func TryScan[T any](s *Scanner, callback func() T) T {
	return scannerSpeculationHelper(s, callback, false)
}

func LookHead[T any](s *Scanner, callback func() T) T {
	return scannerSpeculationHelper(s, callback, true)
}

func ComputeLineStarts(text []byte) []int {
	var result []int
	var pos = 0
	var lineStart = 0
	for pos < len(text) {
		ch, size := utf8.DecodeRune(text[pos:])
		pos += size
		switch ch {
		case '\r':
			if pos+1 < len(text) && text[pos] == '\n' {
				pos++
			}
			fallthrough
		case '\n':
			result = append(result, lineStart)
			lineStart = pos
		default:
			if ch > unicode.MaxASCII && IsLineBreak(ch) {
				result = append(result, lineStart)
				lineStart = pos
			}
		}
	}
	result = append(result, lineStart)
	return result
}

func GetLineStarts(file *SourceCode) []int {
	if file.LineStarts == nil {
		file.LineStarts = ComputeLineStarts(file.Text)
	}
	return file.LineStarts
}

func GetPositionFromLineAndCharacter(text []byte, linetStarts []int, line int, character int) int {
	if line < 0 {
		panic("line < 0")
	}

	offset := linetStarts[line-1]
	for character > 0 {
		if offset >= len(text) {
			panic("offset out of range")
		}
		_, size := utf8.DecodeRune(text[offset:])
		offset += size
		character--
	}

	return offset
}

func GetLineAndCharacterOfPosition(text []byte, lineStarts []int, offset int) Position {
	p, _ := PositionFromOffsetWithCache(offset, text, lineStarts)
	return p
}

func PositionFromOffsetWithCache(offset int, content []byte, lineStarts []int) (Position, error) {
	if offset > len(content) {
		return Position{}, fmt.Errorf("PositionFromOffsetWithCache offset out of content")
	}

	var lineNumber = BinarySearch(lineStarts, offset)
	if lineNumber < 0 {
		lineNumber = ^lineNumber - 1
	}
	return Position{
		Line:   lineNumber,
		Column: offset - lineStarts[lineNumber],
	}, nil
}

func BinarySearch(array []int, value int) int {
	var low = 0
	var high = len(array) - 1

	for low <= high {
		var middle = low + ((high - low) >> 1)
		var midValue = array[middle]

		if midValue == value {
			return middle
		} else if midValue > value {
			high = middle - 1
		} else {
			low = middle + 1
		}
	}

	return ^low
}

func PositionToLineAndCharacter(text []byte, pos int) Position {
	var lineStarts = ComputeLineStarts(text)
	return GetLineAndCharacterOfPosition(text, lineStarts, pos)
}

func IsWhiteSpace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\v' || ch == '\f' ||
		ch == Uni_NonBreakingSpace || ch == Uni_Ogham ||
		ch >= Uni_EnQuad && ch <= Uni_ZeroWidthSpace ||
		ch == Uni_NarrowNoBreakSpace || ch == Uni_MathematicalSpace ||
		ch == Uni_IdeographicSpace || ch == Uni_ByteOrderMark
}

// Does not include line breaks. For that, see isWhiteSpaceLike.
func IsWhiteSpaceSingleLine(ch rune) bool {
	// Note: nextLine is in the Zs space, and should be considered to be a whitespace.
	// It is explicitly not a line-break as it isn't in the exact set specified by EcmaScript.
	return ch == ' ' || ch == '\t' || ch == '\v' || ch == '\f' ||
		ch == Uni_NonBreakingSpace || ch == Uni_NextLine || ch == Uni_Ogham ||
		ch >= Uni_EnQuad && ch <= Uni_ZeroWidthSpace ||
		ch == Uni_NarrowNoBreakSpace || ch == Uni_MathematicalSpace ||
		ch == Uni_IdeographicSpace || ch == Uni_ByteOrderMark
}

func IsLineBreak(ch rune) bool {
	return ch == '\r' || ch == '\n' ||
		ch == Uni_LineSeparator || ch == Uni_ParagraphSeparator ||
		ch == Uni_NextLine
}

func IsDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func IsOctalDigit(ch rune) bool {
	return ch >= '0' && ch <= '7'
}

func IsIdentifierStart(ch rune) bool {
	return ch >= 'A' && ch <= 'Z' ||
		ch >= 'a' && ch <= 'z' ||
		ch == '$' || ch == '_' ||
		ch > unicode.MaxASCII && LookupInUnicodeMap(ch, unicodeES5IdentifierStart)
}

func IsIdentifierPart(ch rune) bool {
	return ch >= 'A' && ch <= 'Z' ||
		ch >= 'a' && ch <= 'z' ||
		ch >= '0' && ch <= '9' ||
		ch == '$' || ch == '_' ||
		ch > unicode.MaxASCII && LookupInUnicodeMap(ch, unicodeES5IdentifierPart)
}

var (
	unicodeES5IdentifierStart = []rune{170, 170, 181, 181, 186, 186, 192, 214, 216, 246, 248, 705, 710, 721, 736, 740, 748, 748, 750, 750, 880, 884, 886, 887, 890, 893, 902, 902, 904, 906, 908, 908, 910, 929, 931, 1013, 1015, 1153, 1162, 1319, 1329, 1366, 1369, 1369, 1377, 1415, 1488, 1514, 1520, 1522, 1568, 1610, 1646, 1647, 1649, 1747, 1749, 1749, 1765, 1766, 1774, 1775, 1786, 1788, 1791, 1791, 1808, 1808, 1810, 1839, 1869, 1957, 1969, 1969, 1994, 2026, 2036, 2037, 2042, 2042, 2048, 2069, 2074, 2074, 2084, 2084, 2088, 2088, 2112, 2136, 2208, 2208, 2210, 2220, 2308, 2361, 2365, 2365, 2384, 2384, 2392, 2401, 2417, 2423, 2425, 2431, 2437, 2444, 2447, 2448, 2451, 2472, 2474, 2480, 2482, 2482, 2486, 2489, 2493, 2493, 2510, 2510, 2524, 2525, 2527, 2529, 2544, 2545, 2565, 2570, 2575, 2576, 2579, 2600, 2602, 2608, 2610, 2611, 2613, 2614, 2616, 2617, 2649, 2652, 2654, 2654, 2674, 2676, 2693, 2701, 2703, 2705, 2707, 2728, 2730, 2736, 2738, 2739, 2741, 2745, 2749, 2749, 2768, 2768, 2784, 2785, 2821, 2828, 2831, 2832, 2835, 2856, 2858, 2864, 2866, 2867, 2869, 2873, 2877, 2877, 2908, 2909, 2911, 2913, 2929, 2929, 2947, 2947, 2949, 2954, 2958, 2960, 2962, 2965, 2969, 2970, 2972, 2972, 2974, 2975, 2979, 2980, 2984, 2986, 2990, 3001, 3024, 3024, 3077, 3084, 3086, 3088, 3090, 3112, 3114, 3123, 3125, 3129, 3133, 3133, 3160, 3161, 3168, 3169, 3205, 3212, 3214, 3216, 3218, 3240, 3242, 3251, 3253, 3257, 3261, 3261, 3294, 3294, 3296, 3297, 3313, 3314, 3333, 3340, 3342, 3344, 3346, 3386, 3389, 3389, 3406, 3406, 3424, 3425, 3450, 3455, 3461, 3478, 3482, 3505, 3507, 3515, 3517, 3517, 3520, 3526, 3585, 3632, 3634, 3635, 3648, 3654, 3713, 3714, 3716, 3716, 3719, 3720, 3722, 3722, 3725, 3725, 3732, 3735, 3737, 3743, 3745, 3747, 3749, 3749, 3751, 3751, 3754, 3755, 3757, 3760, 3762, 3763, 3773, 3773, 3776, 3780, 3782, 3782, 3804, 3807, 3840, 3840, 3904, 3911, 3913, 3948, 3976, 3980, 4096, 4138, 4159, 4159, 4176, 4181, 4186, 4189, 4193, 4193, 4197, 4198, 4206, 4208, 4213, 4225, 4238, 4238, 4256, 4293, 4295, 4295, 4301, 4301, 4304, 4346, 4348, 4680, 4682, 4685, 4688, 4694, 4696, 4696, 4698, 4701, 4704, 4744, 4746, 4749, 4752, 4784, 4786, 4789, 4792, 4798, 4800, 4800, 4802, 4805, 4808, 4822, 4824, 4880, 4882, 4885, 4888, 4954, 4992, 5007, 5024, 5108, 5121, 5740, 5743, 5759, 5761, 5786, 5792, 5866, 5870, 5872, 5888, 5900, 5902, 5905, 5920, 5937, 5952, 5969, 5984, 5996, 5998, 6000, 6016, 6067, 6103, 6103, 6108, 6108, 6176, 6263, 6272, 6312, 6314, 6314, 6320, 6389, 6400, 6428, 6480, 6509, 6512, 6516, 6528, 6571, 6593, 6599, 6656, 6678, 6688, 6740, 6823, 6823, 6917, 6963, 6981, 6987, 7043, 7072, 7086, 7087, 7098, 7141, 7168, 7203, 7245, 7247, 7258, 7293, 7401, 7404, 7406, 7409, 7413, 7414, 7424, 7615, 7680, 7957, 7960, 7965, 7968, 8005, 8008, 8013, 8016, 8023, 8025, 8025, 8027, 8027, 8029, 8029, 8031, 8061, 8064, 8116, 8118, 8124, 8126, 8126, 8130, 8132, 8134, 8140, 8144, 8147, 8150, 8155, 8160, 8172, 8178, 8180, 8182, 8188, 8305, 8305, 8319, 8319, 8336, 8348, 8450, 8450, 8455, 8455, 8458, 8467, 8469, 8469, 8473, 8477, 8484, 8484, 8486, 8486, 8488, 8488, 8490, 8493, 8495, 8505, 8508, 8511, 8517, 8521, 8526, 8526, 8544, 8584, 11264, 11310, 11312, 11358, 11360, 11492, 11499, 11502, 11506, 11507, 11520, 11557, 11559, 11559, 11565, 11565, 11568, 11623, 11631, 11631, 11648, 11670, 11680, 11686, 11688, 11694, 11696, 11702, 11704, 11710, 11712, 11718, 11720, 11726, 11728, 11734, 11736, 11742, 11823, 11823, 12293, 12295, 12321, 12329, 12337, 12341, 12344, 12348, 12353, 12438, 12445, 12447, 12449, 12538, 12540, 12543, 12549, 12589, 12593, 12686, 12704, 12730, 12784, 12799, 13312, 19893, 19968, 40908, 40960, 42124, 42192, 42237, 42240, 42508, 42512, 42527, 42538, 42539, 42560, 42606, 42623, 42647, 42656, 42735, 42775, 42783, 42786, 42888, 42891, 42894, 42896, 42899, 42912, 42922, 43000, 43009, 43011, 43013, 43015, 43018, 43020, 43042, 43072, 43123, 43138, 43187, 43250, 43255, 43259, 43259, 43274, 43301, 43312, 43334, 43360, 43388, 43396, 43442, 43471, 43471, 43520, 43560, 43584, 43586, 43588, 43595, 43616, 43638, 43642, 43642, 43648, 43695, 43697, 43697, 43701, 43702, 43705, 43709, 43712, 43712, 43714, 43714, 43739, 43741, 43744, 43754, 43762, 43764, 43777, 43782, 43785, 43790, 43793, 43798, 43808, 43814, 43816, 43822, 43968, 44002, 44032, 55203, 55216, 55238, 55243, 55291, 63744, 64109, 64112, 64217, 64256, 64262, 64275, 64279, 64285, 64285, 64287, 64296, 64298, 64310, 64312, 64316, 64318, 64318, 64320, 64321, 64323, 64324, 64326, 64433, 64467, 64829, 64848, 64911, 64914, 64967, 65008, 65019, 65136, 65140, 65142, 65276, 65313, 65338, 65345, 65370, 65382, 65470, 65474, 65479, 65482, 65487, 65490, 65495, 65498, 65500}
	unicodeES5IdentifierPart  = []rune{170, 170, 181, 181, 186, 186, 192, 214, 216, 246, 248, 705, 710, 721, 736, 740, 748, 748, 750, 750, 768, 884, 886, 887, 890, 893, 902, 902, 904, 906, 908, 908, 910, 929, 931, 1013, 1015, 1153, 1155, 1159, 1162, 1319, 1329, 1366, 1369, 1369, 1377, 1415, 1425, 1469, 1471, 1471, 1473, 1474, 1476, 1477, 1479, 1479, 1488, 1514, 1520, 1522, 1552, 1562, 1568, 1641, 1646, 1747, 1749, 1756, 1759, 1768, 1770, 1788, 1791, 1791, 1808, 1866, 1869, 1969, 1984, 2037, 2042, 2042, 2048, 2093, 2112, 2139, 2208, 2208, 2210, 2220, 2276, 2302, 2304, 2403, 2406, 2415, 2417, 2423, 2425, 2431, 2433, 2435, 2437, 2444, 2447, 2448, 2451, 2472, 2474, 2480, 2482, 2482, 2486, 2489, 2492, 2500, 2503, 2504, 2507, 2510, 2519, 2519, 2524, 2525, 2527, 2531, 2534, 2545, 2561, 2563, 2565, 2570, 2575, 2576, 2579, 2600, 2602, 2608, 2610, 2611, 2613, 2614, 2616, 2617, 2620, 2620, 2622, 2626, 2631, 2632, 2635, 2637, 2641, 2641, 2649, 2652, 2654, 2654, 2662, 2677, 2689, 2691, 2693, 2701, 2703, 2705, 2707, 2728, 2730, 2736, 2738, 2739, 2741, 2745, 2748, 2757, 2759, 2761, 2763, 2765, 2768, 2768, 2784, 2787, 2790, 2799, 2817, 2819, 2821, 2828, 2831, 2832, 2835, 2856, 2858, 2864, 2866, 2867, 2869, 2873, 2876, 2884, 2887, 2888, 2891, 2893, 2902, 2903, 2908, 2909, 2911, 2915, 2918, 2927, 2929, 2929, 2946, 2947, 2949, 2954, 2958, 2960, 2962, 2965, 2969, 2970, 2972, 2972, 2974, 2975, 2979, 2980, 2984, 2986, 2990, 3001, 3006, 3010, 3014, 3016, 3018, 3021, 3024, 3024, 3031, 3031, 3046, 3055, 3073, 3075, 3077, 3084, 3086, 3088, 3090, 3112, 3114, 3123, 3125, 3129, 3133, 3140, 3142, 3144, 3146, 3149, 3157, 3158, 3160, 3161, 3168, 3171, 3174, 3183, 3202, 3203, 3205, 3212, 3214, 3216, 3218, 3240, 3242, 3251, 3253, 3257, 3260, 3268, 3270, 3272, 3274, 3277, 3285, 3286, 3294, 3294, 3296, 3299, 3302, 3311, 3313, 3314, 3330, 3331, 3333, 3340, 3342, 3344, 3346, 3386, 3389, 3396, 3398, 3400, 3402, 3406, 3415, 3415, 3424, 3427, 3430, 3439, 3450, 3455, 3458, 3459, 3461, 3478, 3482, 3505, 3507, 3515, 3517, 3517, 3520, 3526, 3530, 3530, 3535, 3540, 3542, 3542, 3544, 3551, 3570, 3571, 3585, 3642, 3648, 3662, 3664, 3673, 3713, 3714, 3716, 3716, 3719, 3720, 3722, 3722, 3725, 3725, 3732, 3735, 3737, 3743, 3745, 3747, 3749, 3749, 3751, 3751, 3754, 3755, 3757, 3769, 3771, 3773, 3776, 3780, 3782, 3782, 3784, 3789, 3792, 3801, 3804, 3807, 3840, 3840, 3864, 3865, 3872, 3881, 3893, 3893, 3895, 3895, 3897, 3897, 3902, 3911, 3913, 3948, 3953, 3972, 3974, 3991, 3993, 4028, 4038, 4038, 4096, 4169, 4176, 4253, 4256, 4293, 4295, 4295, 4301, 4301, 4304, 4346, 4348, 4680, 4682, 4685, 4688, 4694, 4696, 4696, 4698, 4701, 4704, 4744, 4746, 4749, 4752, 4784, 4786, 4789, 4792, 4798, 4800, 4800, 4802, 4805, 4808, 4822, 4824, 4880, 4882, 4885, 4888, 4954, 4957, 4959, 4992, 5007, 5024, 5108, 5121, 5740, 5743, 5759, 5761, 5786, 5792, 5866, 5870, 5872, 5888, 5900, 5902, 5908, 5920, 5940, 5952, 5971, 5984, 5996, 5998, 6000, 6002, 6003, 6016, 6099, 6103, 6103, 6108, 6109, 6112, 6121, 6155, 6157, 6160, 6169, 6176, 6263, 6272, 6314, 6320, 6389, 6400, 6428, 6432, 6443, 6448, 6459, 6470, 6509, 6512, 6516, 6528, 6571, 6576, 6601, 6608, 6617, 6656, 6683, 6688, 6750, 6752, 6780, 6783, 6793, 6800, 6809, 6823, 6823, 6912, 6987, 6992, 7001, 7019, 7027, 7040, 7155, 7168, 7223, 7232, 7241, 7245, 7293, 7376, 7378, 7380, 7414, 7424, 7654, 7676, 7957, 7960, 7965, 7968, 8005, 8008, 8013, 8016, 8023, 8025, 8025, 8027, 8027, 8029, 8029, 8031, 8061, 8064, 8116, 8118, 8124, 8126, 8126, 8130, 8132, 8134, 8140, 8144, 8147, 8150, 8155, 8160, 8172, 8178, 8180, 8182, 8188, 8204, 8205, 8255, 8256, 8276, 8276, 8305, 8305, 8319, 8319, 8336, 8348, 8400, 8412, 8417, 8417, 8421, 8432, 8450, 8450, 8455, 8455, 8458, 8467, 8469, 8469, 8473, 8477, 8484, 8484, 8486, 8486, 8488, 8488, 8490, 8493, 8495, 8505, 8508, 8511, 8517, 8521, 8526, 8526, 8544, 8584, 11264, 11310, 11312, 11358, 11360, 11492, 11499, 11507, 11520, 11557, 11559, 11559, 11565, 11565, 11568, 11623, 11631, 11631, 11647, 11670, 11680, 11686, 11688, 11694, 11696, 11702, 11704, 11710, 11712, 11718, 11720, 11726, 11728, 11734, 11736, 11742, 11744, 11775, 11823, 11823, 12293, 12295, 12321, 12335, 12337, 12341, 12344, 12348, 12353, 12438, 12441, 12442, 12445, 12447, 12449, 12538, 12540, 12543, 12549, 12589, 12593, 12686, 12704, 12730, 12784, 12799, 13312, 19893, 19968, 40908, 40960, 42124, 42192, 42237, 42240, 42508, 42512, 42539, 42560, 42607, 42612, 42621, 42623, 42647, 42655, 42737, 42775, 42783, 42786, 42888, 42891, 42894, 42896, 42899, 42912, 42922, 43000, 43047, 43072, 43123, 43136, 43204, 43216, 43225, 43232, 43255, 43259, 43259, 43264, 43309, 43312, 43347, 43360, 43388, 43392, 43456, 43471, 43481, 43520, 43574, 43584, 43597, 43600, 43609, 43616, 43638, 43642, 43643, 43648, 43714, 43739, 43741, 43744, 43759, 43762, 43766, 43777, 43782, 43785, 43790, 43793, 43798, 43808, 43814, 43816, 43822, 43968, 44010, 44012, 44013, 44016, 44025, 44032, 55203, 55216, 55238, 55243, 55291, 63744, 64109, 64112, 64217, 64256, 64262, 64275, 64279, 64285, 64296, 64298, 64310, 64312, 64316, 64318, 64318, 64320, 64321, 64323, 64324, 64326, 64433, 64467, 64829, 64848, 64911, 64914, 64967, 65008, 65019, 65024, 65039, 65056, 65062, 65075, 65076, 65101, 65103, 65136, 65140, 65142, 65276, 65296, 65305, 65313, 65338, 65343, 65343, 65345, 65370, 65382, 65470, 65474, 65479, 65482, 65487, 65490, 65495, 65498, 65500}
)

func LookupInUnicodeMap(code rune, arrMap []rune) bool {
	// Bail out quickly if it couldn't possibly be in the map.
	if code < arrMap[0] {
		return false
	}

	// Perform binary search in one of the Unicode range maps
	lo := 0
	hi := len(arrMap)
	mid := 0

	for lo+1 < hi {
		mid = lo + (hi-lo)/2
		// mid has to be even to cache a range's beginning
		mid -= mid % 2
		if arrMap[mid] <= code && code <= arrMap[mid+1] {
			return true
		}

		if code < arrMap[mid] {
			hi = mid
		} else {
			lo = mid + 2
		}
	}

	return false
}

func GetFileLineAndCharacterFromPosition(file *SourceCode, position int) Position {
	if len(file.LineStarts) == 0 {
		file.LineStarts = GetLineStarts(file)
	}
	return GetLineAndCharacterOfPosition(file.Text, file.LineStarts, position)
}

func GetFilePositionFromLineAndCharacter(file *SourceCode, line int, character int) int {
	if len(file.LineStarts) == 0 {
		file.LineStarts = GetLineStarts(file)
	}
	return GetPositionFromLineAndCharacter(file.Text, file.LineStarts, line, character)
}

func TokenIsIdentifierOrKeyword(tok SyntaxKind) bool {
	return tok >= SK_Identifier
}
