package formula

const (
	Uni_LineSeparator      = 0x2028
	Uni_ParagraphSeparator = 0x2029
	Uni_NextLine           = 0x0085

	// Unicode 3.0 space characters
	Uni_Space              = 0x0020
	Uni_NonBreakingSpace   = 0x00A0
	Uni_EnQuad             = 0x2000
	Uni_EmQuad             = 0x2001
	Uni_EnSpace            = 0x2002
	Uni_EmSpace            = 0x2003
	Uni_ThreePerEmSpace    = 0x2004
	Uni_FourPerEmSpace     = 0x2005
	Uni_SixPerEmSpace      = 0x2006
	Uni_FigureSpace        = 0x2007
	Uni_PunctuationSpace   = 0x2008
	Uni_ThinSpace          = 0x2009
	Uni_HairSpace          = 0x200A
	Uni_ZeroWidthSpace     = 0x200B
	Uni_NarrowNoBreakSpace = 0x202F
	Uni_IdeographicSpace   = 0x3000
	Uni_MathematicalSpace  = 0x205F
	Uni_Ogham              = 0x1680
	Uni_ByteOrderMark      = 0xFEFF
)

type SyntaxKind int

const (
	SK_Unknown SyntaxKind = iota
	SK_EndOfFile

	// Literal
	SK_NumberLiteral
	SK_StringLiteral

	// Punctuation
	SK_OpenParen    // (
	SK_CloseParen   // )
	SK_OpenBracket  // [
	SK_CloseBracket // ]
	SK_Dot          // .
	SK_DotDotDot    // ...
	SK_Comma        // ,

	SK_LessThan                // <
	SK_GreaterThan             // >
	SK_LessThanEquals          // <=
	SK_GreaterThanEquals       // >=
	SK_EqualsEquals            // ==
	SK_EqualsEqualsEquals      // ===
	SK_ExclamationEquals       // !=
	SK_ExclamationEqualsEquals // !==
	SK_Plus                    // +
	SK_Minus                   // -
	SK_Asterisk                // *
	SK_Slash                   // /
	SK_Percent                 // %
	SK_Ampersand               // &
	SK_Bar                     // |
	SK_Caret                   // ^
	SK_AmpersandAmpersand      // &&
	SK_BarBar                  // ||
	SK_QuestionQuestion        // ??
	SK_Exclamation             // !
	SK_ExclamationDot          // !.
	SK_ExclamationExclamation  // !!
	SK_Tilde                   // ~
	SK_Question                // ?
	SK_Colon                   // :

	// Assignments
	SK_Equals                                  // =
	SK_PlusEquals                              // +=
	SK_MinusEquals                             // -=
	SK_AsteriskEquals                          // *=
	SK_SlashEquals                             // /=
	SK_PercentEquals                           // %=
	SK_LessThanLessThanEquals                  // <<=
	SK_GreaterThanGreaterThanEquals            // >>=
	SK_GreaterThanGreaterThanGreaterThanEquals // >>>=
	SK_AmpersandEquals                         // &=
	SK_BarEquals                               // |=
	SK_CaretEquals                             // ^=

	// Identifiers
	SK_Identifier

	// Keyword
	SK_TrueKeyword
	SK_FalseKeyword
	SK_NullKeyword
	SK_ThisKeyword
	SK_CtxKeyword
	SK_TypeofKeyword

	SK_Count
	// Markers
	SK_FirstAssignment     = SK_Equals
	SK_LastAssignment      = SK_CaretEquals
	SK_FirstKeyword        = SK_TrueKeyword
	SK_LastKeyword         = SK_TypeofKeyword
	SK_FirstPunctuation    = SK_OpenParen
	SK_LastPunctuation     = SK_Comma
	SK_FirstLiteral        = SK_NumberLiteral
	SK_LastLiteral         = SK_StringLiteral
	SK_FirstBinaryOperator = SK_LessThan
	SK_LastBinaryOperator  = SK_QuestionQuestion
)

var tokens = [...]string{
	// Punctuation
	SK_OpenParen:    "(",
	SK_CloseParen:   ")",
	SK_OpenBracket:  "[",
	SK_CloseBracket: "]",
	SK_Dot:          ".",
	SK_Comma:        ",",
	// Keyword
	SK_TrueKeyword:   "true",
	SK_FalseKeyword:  "false",
	SK_NullKeyword:   "null",
	SK_ThisKeyword:   "this",
	SK_CtxKeyword:    "ctx",
	SK_TypeofKeyword: "typeof",
}

func (tok SyntaxKind) IsAssignmentOperator() bool {
	return tok >= SK_FirstAssignment && tok <= SK_LastAssignment
}

func (tok SyntaxKind) IsKeyword() bool { return tok >= SK_FirstKeyword && tok <= SK_LastKeyword }

func (tok SyntaxKind) IsPunctuation() bool {
	return tok >= SK_FirstPunctuation && tok <= SK_LastPunctuation
}

func (tok SyntaxKind) IsBinaryOperator() bool {
	return tok >= SK_FirstBinaryOperator && tok <= SK_LastBinaryOperator
}

func (tok SyntaxKind) IsLiteral() bool { return tok >= SK_FirstLiteral && tok <= SK_LastLiteral }

func (tok SyntaxKind) IsIdentifier() bool {
	if tok == SK_Identifier {
		return true
	}
	return tok.IsKeyword()
}

func (tok SyntaxKind) ToString() string { return tokens[tok] }

var keywords map[string]SyntaxKind

func init() {
	keywords = make(map[string]SyntaxKind)
	for i := SK_FirstKeyword; i <= SK_LastKeyword; i++ {
		keywords[tokens[i]] = i
	}
}

func KeywordFromString(text string) SyntaxKind {
	tok, ok := keywords[text]
	if ok {
		return tok
	}
	return SK_Unknown
}

// Position represents a single point within a file.
// In general this should only be used as part of a Span, as on its own it
// does not carry enough information.
type Position struct {
	Line   int
	Column int
}

type TextRange interface {
	Pos() int
	End() int
	SetPos(pos int)
	SetEnd(end int)
}

type textRange struct {
	pos int
	end int
}

func (t *textRange) Pos() int       { return t.pos }
func (t *textRange) End() int       { return t.end }
func (t *textRange) SetPos(pos int) { t.pos = pos }
func (t *textRange) SetEnd(end int) { t.end = end }

type List interface {
	Pos() int
	End() int
	NodeAt(index int) Node
	Len() int
}

type NodeList[T any] struct {
	nodes []T
	textRange
}

func (nl *NodeList[T]) Add(node T)            { nl.nodes = append(nl.nodes, node) }
func (nl *NodeList[T]) At(index int) T        { return nl.nodes[index] }
func (nl *NodeList[T]) NodeAt(index int) Node { var r any = nl.nodes[index]; return r.(Node) }
func (nl *NodeList[T]) Index(node Node) int {
	for i, t := range nl.nodes {
		var c1 any = node
		var c2 any = t
		if c1 == c2 {
			return i
		}
	}
	return -1
}
func (nl *NodeList[T]) Len() int {
	if nl == nil {
		return 0
	}
	return len(nl.nodes)
}
func (nl *NodeList[T]) Array() []T {
	if nl != nil {
		return nl.nodes
	}
	return nil
}

type Node interface {
	TextRange
	ID() int
	SetID(id int)
	Parent() Node
	SetParent(node Node)
	aNode()
}

type Expression interface {
	Node
	aExpression()
}

// Node
type node struct {
	id     int // Unique ID (used to look up NodeLinks)
	parent Node
	textRange
}

func (n *node) ID() int             { return n.id }
func (n *node) SetID(id int)        { n.id = id }
func (n *node) Parent() Node        { return n.parent }
func (n *node) SetParent(node Node) { n.parent = node }
func (n *node) aNode()              {}

type TokenNode struct {
	Token SyntaxKind
	node
}

// Expr
type expression struct{ node }

func (e expression) aExpression() {}

type (
	Identifier struct {
		Value         string
		OriginalToken SyntaxKind
		expression
	}

	// !endswith(Description,'milk')
	PrefixUnaryExpression struct {
		Operator *TokenNode
		Operand  Expression
		expression
	}

	TypeOfExpression struct {
		Expression Expression
		expression
	}

	// name == 'value'
	BinaryExpression struct {
		Left     Expression
		Operator *TokenNode
		Right    Expression
		expression
	}

	// Cond ? True : False
	ConditionalExpression struct {
		Condition   Expression
		QuestionTok *TokenNode
		WhenTrue    Expression
		ColonTok    *TokenNode
		WhenFalse   Expression
		expression
	}

	// [0, 1, 2]
	ArrayLiteralExpression struct {
		Elements *NodeList[Expression]
		expression
	}

	// (Expression)
	ParenthesizedExpression struct {
		Expression Expression
		expression
	}

	// "str" 10 10L
	LiteralExpression struct {
		Token SyntaxKind
		Value string
		expression
	}

	// Expression.Name
	SelectorExpression struct {
		Expression Expression
		Name       *Identifier
		Assert     bool
		expression
	}

	// Expression(Arguments)
	CallExpression struct {
		Expression     Expression
		Arguments      *NodeList[Expression]
		DotDotDotToken *TokenNode
		expression
	}
)

type SourceCode struct {
	Text []byte

	EndOfFileToken *TokenNode

	NodeCount       int
	IdentifierCount int
	LineStarts      []int
	Diagnostics     []*Diagnostic
	Expression      Expression

	node
}

type DiagnosticCategory int

const (
	Warning DiagnosticCategory = iota
	Error
	Information
)

func (k DiagnosticCategory) ToString() string {
	switch k {
	case Warning:
		return "warning"
	case Error:
		return "error"
	case Information:
		return "info"
	default:
		return "unknown"
	}
}

type DiagnosticMessage struct {
	Code     int
	Category DiagnosticCategory
	Message  string
}

// A linked list of formatted diagnostic messages to be used as part of a multiline message.
// It is built from the bottom up, leaving the head to be the "main"
// While it seems that MessageChain is structurally similar to Message,
// the difference is that messages are all preformatted in DMC.
type MessageChain struct {
	MessageText string
	Category    DiagnosticCategory
	Code        int
	Next        *MessageChain
}

type Diagnostic struct {
	File        *SourceCode
	Start       int
	Length      int
	Category    DiagnosticCategory
	Code        int
	MessageText string
}
