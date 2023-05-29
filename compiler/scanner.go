package compiler

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
				s.pos = tar
				s.token = SK_ExclamationEquals
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
		ch == '$' || ch == '_'
}

func IsIdentifierPart(ch rune) bool {
	return ch >= 'A' && ch <= 'Z' ||
		ch >= 'a' && ch <= 'z' ||
		ch >= '0' && ch <= '9' ||
		ch == '$' || ch == '_'
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
