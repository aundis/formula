package formula

import (
	"errors"
	"fmt"
	"runtime"
)

type parsingContext = int

const (
	pcArgumentExpressions parsingContext = iota // Expressions in argument list
	pcArrayLiteralMembers                       // Members in array literal
	pcParsingContextCount                       // Number of parsing contexts
)

type Parser struct {
	scanner *Scanner

	sourceCode       *SourceCode
	parseDiagnostics []*Diagnostic

	sourceText      []byte
	nodeCount       int
	identifierCount int

	parsingCtx parsingContext

	// hasDeprecatedTag bool
}

func ParseSourceCode(content []byte) (source *SourceCode, err error) {
	defer func() {
		capture := recover()
		if capture != nil {
			switch err.(type) {
			case runtime.Error:
				err = errors.New("runtime error")
			default:
				err = fmt.Errorf("%v", capture)
			}
		}
		if source != nil && len(source.Diagnostics) > 0 {
			err = errors.New(FormatDiagnostic(source, source.Diagnostics[0]))
		}
	}()

	parser := &Parser{
		sourceText:       content,
		sourceCode:       nil,
		parseDiagnostics: nil,
		nodeCount:        0,
		identifierCount:  0,
		parsingCtx:       0,
	}
	source = parser.parseSourceFileWorker(content)
	return
}

func (p *Parser) startPos() int {
	return p.scanner.GetStartPos()
}

func finishNode[T Node](p *Parser, node T, pos ...int) T {
	p.nodeCount++
	node.SetPos(pos[0])
	if len(pos) > 1 {
		node.SetEnd(pos[1])
	} else {
		node.SetEnd(p.startPos())
	}
	return node
}

func (p *Parser) fillMissPos(node TextRange) {
	node.SetPos(p.startPos())
	node.SetEnd(p.startPos())
}

func (p *Parser) nextTokenIsIdentifierOrKeywordOnSameLine() bool {
	p.nextToken()
	return TokenIsIdentifierOrKeyword(p.token()) && !p.scanner.HasPrecedingLineBreak()
}

func (p *Parser) parseSourceFileWorker(content []byte) *SourceCode {
	p.sourceCode = p.createSourceCode(content)
	p.scanner = CreateScanner(p.sourceText, p.scanError)
	// Prime the scanner.
	p.nextToken()
	// parse expression list
	p.sourceCode.Expression = p.parseExpression()
	assertMsg(p.token() == SK_EndOfFile, fmt.Sprintf("End of file not reached, stop at %d(\"%s\")", p.scanner.pos, p.scanner.GetTokenText()))
	p.sourceCode.EndOfFileToken = p.parseToken()
	// 记录相关信息
	p.sourceCode.NodeCount = p.nodeCount
	p.sourceCode.IdentifierCount = p.identifierCount
	p.sourceCode.Diagnostics = p.parseDiagnostics
	return p.sourceCode
}

func (p *Parser) createSourceCode(content []byte) *SourceCode {
	// code from createNode is inlined here so createNode won't have to deal with special case of creating session files
	// this is quite rare comparing to other nodes and createNode should be as fast as possible
	var sourceFile = new(SourceCode)
	sourceFile.SetPos(0)
	sourceFile.SetEnd(len(p.sourceText))
	p.nodeCount++

	sourceFile.Text = p.sourceText

	return sourceFile
}

func (p *Parser) errorAtCurrentToken(message *DiagnosticMessage, args ...interface{}) {
	var start = p.scanner.GetTokenPos()
	var length = p.scanner.GetTextPos() - start

	p.errorAtPosition(start, length, message, args...)
}

func (p *Parser) errorAtPosition(start int, length int, message *DiagnosticMessage, args ...interface{}) {
	// Don't report another error if it would just be at the same position as the last error.
	if n := len(p.parseDiagnostics); n == 0 || start != p.parseDiagnostics[n-1].Start {
		p.parseDiagnostics = append(p.parseDiagnostics, CreateFileDiagnostic(p.sourceCode, start, length, message, args...))
	}
}

func (p *Parser) scanError(message *DiagnosticMessage, pos int, length int) {
	if pos == -1 {
		pos = p.scanner.GetTextPos()
	}
	p.errorAtPosition(pos, length, message)
}

func (p *Parser) getNodePos() int {
	return p.scanner.GetStartPos()
}

func (p *Parser) getNodeEnd() int {
	return p.scanner.GetStartPos()
}

func (p *Parser) token() SyntaxKind {
	return p.scanner.GetToken()
}

func (p *Parser) nextToken() SyntaxKind {
	p.scanner.Scan()
	return p.token()
}

func parserSpeculationHelper[T any](p *Parser, callback func() T, isLookAhead bool) T {
	// Keep track of the state we'll need to rollback to if lookahead fails (or if the
	// caller asked us to always reset our state).
	var saveSyntacticErrorsLength = len(p.parseDiagnostics)

	var result T
	if isLookAhead {
		result = LookHead(p.scanner, callback)
	} else {
		result = TryScan(p.scanner, callback)
	}

	// If our callback returned something 'falsy' or we're just looking ahead,
	// then unconditionally restore us to where we were.
	if IsNull(result) || isLookAhead {
		p.parseDiagnostics = p.parseDiagnostics[:saveSyntacticErrorsLength]
	}

	return result
}

// Invokes the provided callback then unconditionally restores the parser to the state it
// was in immediately prior to invoking the callback.  The result of invoking the callback
// is returned from this function.
func lookAhead[T any](p *Parser, callback func() T) T {
	return parserSpeculationHelper(p, callback, true)
}

// Invokes the provided callback.  If the callback returns something falsy, then it restores
// the parser to the state it was in immediately prior to invoking the callback.  If the
// callback returns something truthy, then the parser state is not rolled back.  The result
// of invoking the callback is returned from this function.
func tryParse[T any](p *Parser, callback func() T) T {
	return parserSpeculationHelper(p, callback, false)
}

func (p *Parser) want(t SyntaxKind) bool {
	return p.parseExpected(t, nil, true)
}

func (p *Parser) parseExpected(kind SyntaxKind, diagnosticMessage *DiagnosticMessage, shouldAdvance bool) bool {
	if p.token() == kind {
		if shouldAdvance {
			p.nextToken()
		}
		return true
	}

	// Report specific message if provided with one.  Otherwise, report generic fallback message.
	if diagnosticMessage != nil {
		p.errorAtCurrentToken(diagnosticMessage)
	} else {
		p.errorAtCurrentToken(M_0_expected, kind.ToString())
	}
	return false
}

func (p *Parser) got(t SyntaxKind) bool {
	if p.token() == t {
		p.nextToken()
		return true
	}
	return false
}

func (p *Parser) gotToken(t SyntaxKind) *TokenNode {
	if p.token() == t {
		return p.parseToken()
	}
	return nil
}

func (p *Parser) wantToken(t SyntaxKind, reportAtCurrentPosition bool, diagnosticMessage *DiagnosticMessage, args ...interface{}) *TokenNode {
	if t := p.gotToken(t); t != nil {
		return t
	}

	if reportAtCurrentPosition {
		p.errorAtPosition(p.startPos(), 0, diagnosticMessage, args)
	} else {
		p.errorAtCurrentToken(diagnosticMessage, args)
	}

	// Missing token (pos == end)
	var node = new(TokenNode)
	p.fillMissPos(node)
	return node
}

// An identifier that starts with two underscores has an extra underscore character prepended to it to avoid issues
// with magic property names like '__proto__'. The 'identifiers' object is used to share a single string instance for
// each identifier in order to reduce memory consumption.
func (p *Parser) createIdentifier(isIdentifier bool, diagnosticMessage *DiagnosticMessage) *Identifier {
	if isIdentifier {
		p.identifierCount++
		var node = new(Identifier)
		var pos = p.getNodePos()
		node.OriginalToken = p.token()
		node.Value = p.scanner.GetTokenValue()
		p.nextToken()
		return finishNode(p, node, pos)
	}

	if diagnosticMessage == nil {
		diagnosticMessage = M_Identifier_expected
	}
	p.errorAtCurrentToken(diagnosticMessage)

	var node = new(Identifier)
	p.fillMissPos(node)
	return node
}

func (p *Parser) parseIdentifier(diagnosticMessage *DiagnosticMessage) *Identifier {
	return p.createIdentifier(p.token().IsIdentifier(), diagnosticMessage)
}

// True if positioned at the start of a list element
func (p *Parser) isListElement(context parsingContext) bool {
	switch context {
	case pcArgumentExpressions:
		return p.isStartOfExpression()
	case pcArrayLiteralMembers:
		return p.token() == SK_Comma || p.isStartOfExpression()
	}

	panic("Non-exhaustive case in 'isListElement'.")
}

func (p *Parser) nextTokenIsIdentifier() bool {
	p.nextToken()
	return p.token().IsIdentifier()
}

func (p *Parser) nextTokenIsIdentifierOrKeyword() bool {
	p.nextToken()
	return TokenIsIdentifierOrKeyword(p.token())
}

func (p *Parser) nextTokenIsStartOfExpression() bool {
	p.nextToken()
	return p.isStartOfExpression()
}

// True if positioned at a list terminator
func (p *Parser) isListTerminator(kind parsingContext) bool {
	if p.token() == SK_EndOfFile {
		// Being at the end of the sourceFile ends all lists.
		return true
	}

	switch kind {
	case pcArgumentExpressions:
		// Tokens other than ')' are here for better error recovery
		return p.token() == SK_CloseParen || p.token() == SK_DotDotDot
	case pcArrayLiteralMembers:
		return p.token() == SK_CloseBracket
	}
	return false
}

// Parses a list of elements
func parseList[T Node](p *Parser, kind parsingContext, parseElement func() T) *NodeList[T] {
	saveParsingCtx := p.parsingCtx
	p.parsingCtx |= 1 << kind

	var list *NodeList[T]
	list.SetPos(p.getNodePos())
	for !p.isListTerminator(kind) {
		if p.isListElement(kind) {
			if list == nil {
				list = new(NodeList[T])
			}
			list.Add(parseElement())
			continue
		}

		if p.reportErrorAndMoveToNextToken(kind) {
			break
		}
	}

	p.parsingCtx = saveParsingCtx
	list.SetEnd(p.getNodePos())
	return list
}

func (p *Parser) parseListElement(_ parsingContext, parseElement func() *Node) *Node {
	return parseElement()
}

// Returns true if we should abort parsing.
func (p *Parser) reportErrorAndMoveToNextToken(kind parsingContext) bool {
	p.errorAtCurrentToken(parsingContextErrors(kind))
	p.nextToken()
	return false
}

func parsingContextErrors(context parsingContext) *DiagnosticMessage {
	switch context {
	case pcArgumentExpressions:
		return M_Argument_expression_expected
	case pcArrayLiteralMembers:
		return M_Expression_or_comma_expected
	}

	panic(fmt.Sprintf("ParsingContext(%d) kind is unknown:", context))
}

// Parses a comma-delimited list of elements
func parseDelimitedList[T Node](p *Parser, kind parsingContext, parseElement func() T, allowTrailingComma bool) *NodeList[T] {
	var saveParsingContext = p.parsingCtx
	p.parsingCtx |= 1 << kind

	var list = new(NodeList[T])
	list.SetPos(p.getNodePos())
	var commaStart = -1 // Meaning the previous token was not a comma
	for {
		if p.isListElement(kind) {
			list.Add(parseElement())
			commaStart = p.scanner.GetTokenPos()
			if p.got(SK_Comma) {
				continue
			}

			commaStart = -1 // Back to the state where the last token was not a comma
			if p.isListTerminator(kind) {
				break
			}

			// We didn't get a comma, and the list wasn't terminated, explicitly parse
			// out a comma so we give a good error message.
			p.parseExpected(SK_Comma, nil, true)

			continue
		}

		if p.isListTerminator(kind) {
			break
		}

		if p.reportErrorAndMoveToNextToken(kind) {
			break
		}
	}

	// Recording the trailing comma is deliberately done after the previous
	// loop, and not just if we see a list terminator. This is because the list
	// may have ended incorrectly, but it is still important to know if there
	// was a trailing comma.
	// Check if the last token was a comma.
	if commaStart >= 0 && !allowTrailingComma {
		p.errorAtCurrentToken(M_Trailing_comma_not_allowed)
	}

	p.parsingCtx = saveParsingContext
	list.SetEnd(p.getNodePos())
	return list
}

func (p *Parser) parseRightSideOfDot() *Identifier {
	// Technically a keyword is valid here as all identifiers and keywords are identifier names.
	// However, often we'll encounter this in error situations when the identifier or keyword
	// is actually starting another valid construct.
	//
	// So, we check for the following specific case:
	//
	//      name.
	//      identifierOrKeyword identifierNameOrKeyword
	//
	// Note: the newlines are important here.  For example, if that above code
	// were rewritten into:
	//
	//      name.identifierOrKeyword
	//      identifierNameOrKeyword
	//
	// Then we would consider it valid.  That's because ASI would take effect and
	// the code would be implicitly: "name.identifierOrKeyword; identifierNameOrKeyword".
	// In the first case though, ASI will not take effect because there is not a
	// line terminator after the identifier or keyword.
	if p.scanner.HasPrecedingLineBreak() && TokenIsIdentifierOrKeyword(p.token()) {
		var matchesPattern = lookAhead(p, p.nextTokenIsIdentifierOrKeywordOnSameLine)

		if matchesPattern {
			// Report that we need an identifier.  However, report it right after the dot,
			// and not on the next SK_  This is because the next token might actually
			// be an identifier and the error would be quite confusing.
			var node = new(Identifier)
			node.SetPos(p.startPos())
			node.SetEnd(p.startPos())
			p.errorAtPosition(p.startPos(), 0, M_Identifier_expected)
			return node
		}
	}

	return p.parseIdentifier(nil)
}

func (p *Parser) parseLiteralExpression() *LiteralExpression {
	return p.parseLiteralExpressionRest(p.token())
}

func (p *Parser) parseLiteralExpressionRest(kind SyntaxKind) *LiteralExpression {
	var pos = p.getNodePos()
	var node = new(LiteralExpression)
	node.Token = kind
	node.Value = p.scanner.GetTokenValue()
	p.nextToken()
	return finishNode(p, node, pos)
}

func (p *Parser) parseToken() *TokenNode {
	var pos = p.getNodePos()
	var node = new(TokenNode)
	node.Token = p.token()
	p.nextToken()
	return finishNode(p, node, pos)
}

// EXPRESSIONS

func (p *Parser) isStartOfLeftHandSideExpression() bool {
	tok := p.token()
	switch tok {
	case SK_TrueKeyword,
		SK_FalseKeyword,
		// SK_IntLiteral,
		// SK_LongLiteral,
		// SK_FloatLiteral,
		// SK_DoubleLiteral,
		SK_NumberLiteral,
		SK_StringLiteral,
		SK_OpenParen,
		SK_OpenBracket,
		SK_Slash,
		SK_Identifier:
		return true
	default:
		return tok.IsIdentifier()
	}
}

func (p *Parser) isStartOfExpression() bool {
	if p.isStartOfLeftHandSideExpression() {
		return true
	}

	tok := p.token()
	switch tok {
	case SK_Plus,
		SK_Minus,
		SK_Tilde,
		SK_Exclamation,
		SK_LessThan:
		return true
	default:
		// Error tolerance.  If we see the start of some binary operator, we consider
		// that the start of an expression.  That way we'll parse out a missing identifier,
		// give a good message about an identifier being missing, and then consume the
		// rest of the binary expression.
		if p.isBinaryOperator() {
			return true
		}

		return tok.IsIdentifier()
	}
}

func (p *Parser) parseExpression() Expression {
	var expr = p.parseAssignmentExpressionOrHigher()
	// Comma expression
	for {
		operatorToken := p.gotToken(SK_Comma)
		if operatorToken == nil {
			break
		}
		expr = p.makeBinaryExpression(expr, operatorToken, p.parseAssignmentExpressionOrHigher())
	}
	return expr
}

func (p *Parser) parseAssignmentExpressionOrHigher() Expression {
	var expr = p.parseBinaryExpression(0)
	if p.token().IsAssignmentOperator() {
		return p.makeBinaryExpression(expr, p.parseToken(), p.parseAssignmentExpressionOrHigher())
	}
	return p.parseConditionalExpression(expr)
}

func (p *Parser) parseConditionalExpression(leftOperand Expression) Expression {
	// Note: we are passed in an expression which was produced from parseBinaryExpressionOrHigher.
	var questionToken = p.gotToken(SK_Question)
	if questionToken == nil {
		return leftOperand
	}

	// Note: we explicitly 'allowIn' in the whenTrue part of the condition expression, and
	// we do not that for the 'whenFalse' part.
	var node = new(ConditionalExpression)
	node.Condition = leftOperand
	node.QuestionTok = questionToken
	node.WhenTrue = p.parseAssignmentExpressionOrHigher()
	node.ColonTok = p.wantToken(SK_Colon, false,
		M_0_expected, SK_Colon.ToString())
	node.WhenFalse = p.parseAssignmentExpressionOrHigher()
	finishNode(p, node, leftOperand.Pos())
	return node
}

func (p *Parser) parseBinaryExpression(precedence int) Expression {
	var leftOperand = p.parseUnaryExpression()
	return p.parseBinaryExpressionRest(precedence, leftOperand)
}

func (p *Parser) parseBinaryExpressionRest(precedence int, leftOperand Expression) Expression {
	for {
		// We either have a binary operator here, or we're finished.  We call
		var newPrecedence = p.getBinaryOperatorPrecedence()

		// Check the precedence to see if we should "take" this operator
		// - For left associative operator (all operator but **), consume the operator,
		//   recursively call the function below, and parse binaryExpression as a rightOperand
		//   of the caller if the new precedence of the operator is greater then or equal to the current precedence.
		//   For example:
		//      a - b - c
		//            ^token; leftOperand = b. Return b to the caller as a rightOperand
		//      a * b - c
		//            ^token; leftOperand = b. Return b to the caller as a rightOperand
		//      a - b * c
		//            ^token; leftOperand = b. Return b * c to the caller as a rightOperand
		var consumeCurrentOperator = newPrecedence > precedence

		if !consumeCurrentOperator {
			break
		}

		leftOperand = p.makeBinaryExpression(leftOperand, p.parseToken(), p.parseBinaryExpression(newPrecedence))
	}

	return leftOperand
}

func (p *Parser) isBinaryOperator() bool {
	return p.getBinaryOperatorPrecedence() > 0
}

func (p *Parser) getBinaryOperatorPrecedence() int {
	switch p.token() {
	case SK_BarBar,
		SK_QuestionQuestion:
		return 1
	case SK_AmpersandAmpersand:
		return 2
	case SK_Bar:
		return 3
	case SK_Caret:
		return 4
	case SK_Ampersand:
		return 5
	case SK_EqualsEquals,
		SK_EqualsEqualsEquals,
		SK_ExclamationEquals,
		SK_ExclamationEqualsEquals:
		return 6
	case SK_LessThan,
		SK_GreaterThan,
		SK_LessThanEquals,
		SK_GreaterThanEquals:
		return 7
	// case SK_LessThanLessThan,
	// 	SK_GreaterThanGreaterThan,
	// 	SK_GreaterThanGreaterThanGreaterThan:
	// 	return 8
	case SK_Plus,
		SK_Minus:
		return 9
	case SK_Asterisk,
		SK_Slash,
		SK_Percent:
		return 10
	}

	// -1 is lower than all other precedences.  Returning it will cause binary expression
	// parsing to stop.
	return -1
}

func (p *Parser) makeBinaryExpression(left Expression, operator *TokenNode, right Expression) *BinaryExpression {
	var node = new(BinaryExpression)
	node.Left = left
	node.Operator = operator
	node.Right = right
	return finishNode(p, node, left.Pos())
}

func (p *Parser) parsePrefixUnaryExpression() *PrefixUnaryExpression {
	var pos = p.getNodePos()
	var node = new(PrefixUnaryExpression)
	node.Operator = p.parseToken()
	node.Operand = p.parseSimpleUnaryExpression()
	return finishNode(p, node, pos)
}

func (p *Parser) parseTypeOfExpression() *TypeOfExpression {
	var pos = p.getNodePos()
	var node = new(TypeOfExpression)
	p.nextToken()
	node.Expression = p.parseSimpleUnaryExpression()
	return finishNode(p, node, pos)
}

func (p *Parser) parseUnaryExpression() Expression {
	return p.parseSimpleUnaryExpression()
}

// Parse simple-unary expression or higher:
//
// UnaryExpression:
//  1. UpdateExpression
//  3. + UnaryExpression
//  4. - UnaryExpression
func (p *Parser) parseSimpleUnaryExpression() Expression {
	switch p.token() {
	case SK_Plus,
		SK_Minus,
		SK_Tilde,
		SK_Exclamation,
		SK_ExclamationExclamation:
		return p.parsePrefixUnaryExpression()
	case SK_TypeofKeyword:
		return p.parseTypeOfExpression()
	default:
		return p.parseLeftHandSideExpressionOrHigher()
	}
}

func (p *Parser) parseLeftHandSideExpressionOrHigher() Expression {
	var expression = p.parseMemberExpressionOrHigher()
	return p.parseCallExpressionRest(expression)
}

func (p *Parser) parseMemberExpressionOrHigher() Expression {
	var expression = p.parsePrimaryExpression()
	return p.parseMemberExpressionRest(expression)
}

func (p *Parser) parseMemberExpressionRest(expr Expression) Expression {
	for {
		// Must on same line
		if p.scanner.HasPrecedingLineBreak() {
			break
		}

		dotToken := p.gotToken(SK_Dot)
		exclamationDot := p.gotToken(SK_ExclamationDot)
		if dotToken != nil || exclamationDot != nil {
			var node = new(SelectorExpression)
			node.Expression = expr
			node.Name = p.parseRightSideOfDot()
			node.Assert = exclamationDot != nil
			expr = finishNode(p, node, expr.Pos())
			continue
		}

		break
	}

	return expr
}

func (p *Parser) parseCallExpressionRest(expr Expression) Expression {
	for {
		// Must on same line
		if p.scanner.HasPrecedingLineBreak() {
			break
		}

		expr = p.parseMemberExpressionRest(expr)
		if p.token() == SK_OpenParen {
			var callExpr = new(CallExpression)
			callExpr.Expression = expr
			callExpr.Arguments, callExpr.DotDotDotToken = p.parseArgumentList()
			expr = finishNode(p, callExpr, expr.Pos())
			continue
		}

		break
	}

	return expr
}

func (p *Parser) parseArgumentList() (*NodeList[Expression], *TokenNode) {
	if !p.want(SK_OpenParen) {
		return nil, nil
	}
	var list = parseDelimitedList(p, pcArgumentExpressions, p.parseArgumentExpression, false)
	var token *TokenNode
	if p.token() == SK_DotDotDot {
		token = p.parseToken()
	}
	p.want(SK_CloseParen)
	return list, token
}

func (p *Parser) parsePrimaryExpression() Expression {
	switch p.token() {
	case SK_NumberLiteral,
		SK_StringLiteral,
		SK_NullKeyword,
		SK_TrueKeyword,
		SK_FalseKeyword,
		SK_ThisKeyword,
		SK_CtxKeyword:
		return p.parseLiteralExpression()
	case SK_OpenParen:
		return p.parseParenthesizedExpression()
	case SK_OpenBracket:
		return p.parseArrayLiteralExpression()
	}

	return p.parseIdentifier(M_Expression_expected)
}

func (p *Parser) parseParenthesizedExpression() *ParenthesizedExpression {
	var pos = p.getNodePos()
	var node = new(ParenthesizedExpression)
	p.want(SK_OpenParen)
	node.Expression = p.parseExpression()
	p.want(SK_CloseParen)
	return finishNode(p, node, pos)
}

func (p *Parser) parseArgumentOrArrayLiteralElement() Expression {
	return p.parseAssignmentExpressionOrHigher()
}

func (p *Parser) parseArgumentExpression() Expression {
	return p.parseArgumentOrArrayLiteralElement()
}

func (p *Parser) parseArrayLiteralExpression() *ArrayLiteralExpression {
	var pos = p.getNodePos()
	var node = new(ArrayLiteralExpression)
	p.want(SK_OpenBracket)
	var list = parseDelimitedList(p, pcArrayLiteralMembers, p.parseArgumentOrArrayLiteralElement, false)
	node.Elements = list
	p.want(SK_CloseBracket)
	return finishNode(p, node, pos)
}
