package db

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// tokenType represents lexical tokens.
type tokenType int

const (
	tokEOF tokenType = iota
	tokError
	tokIdent
	tokString
	tokNumber
	tokTrue
	tokFalse
	tokNull
	tokAnd
	tokOr
	tokEq
	tokNe
	tokNot
	tokLParen
	tokRParen
	tokDot
)

type token struct {
	typ tokenType
	val string
}

// Tokenize parses an expression string into a slice of tokens.
func Tokenize(input string) []token {
	var tokens []token
	runes := []rune(input)
	i := 0
	n := len(runes)

	for i < n {
		r := runes[i]
		if unicode.IsSpace(r) {
			i++
			continue
		}

		if r == '(' {
			tokens = append(tokens, token{tokLParen, "("})
			i++
			continue
		}
		if r == ')' {
			tokens = append(tokens, token{tokRParen, ")"})
			i++
			continue
		}
		if r == '.' {
			tokens = append(tokens, token{tokDot, "."})
			i++
			continue
		}
		if r == '!' {
			if i+1 < n && runes[i+1] == '=' {
				tokens = append(tokens, token{tokNe, "!="})
				i += 2
			} else {
				tokens = append(tokens, token{tokNot, "!"})
				i++
			}
			continue
		}
		if r == '=' {
			if i+1 < n && runes[i+1] == '=' {
				tokens = append(tokens, token{tokEq, "=="})
				i += 2
			} else {
				tokens = append(tokens, token{tokError, "="})
				i++
			}
			continue
		}
		if r == '&' && i+1 < n && runes[i+1] == '&' {
			tokens = append(tokens, token{tokAnd, "&&"})
			i += 2
			continue
		}
		if r == '|' && i+1 < n && runes[i+1] == '|' {
			tokens = append(tokens, token{tokOr, "||"})
			i += 2
			continue
		}

		// String literals
		if r == '"' || r == '\'' {
			quote := r
			start := i + 1
			i++
			for i < n && runes[i] != quote {
				i++
			}
			if i < n {
				tokens = append(tokens, token{tokString, string(runes[start:i])})
				i++
			} else {
				tokens = append(tokens, token{tokError, "unterminated string"})
			}
			continue
		}

		// Numbers
		if unicode.IsDigit(r) {
			start := i
			for i < n && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			tokens = append(tokens, token{tokNumber, string(runes[start:i])})
			continue
		}

		// Identifiers and keywords
		if unicode.IsLetter(r) || r == '_' {
			start := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			val := string(runes[start:i])
			switch val {
			case "true":
				tokens = append(tokens, token{tokTrue, "true"})
			case "false":
				tokens = append(tokens, token{tokFalse, "false"})
			case "null":
				tokens = append(tokens, token{tokNull, "null"})
			default:
				tokens = append(tokens, token{tokIdent, val})
			}
			continue
		}

		tokens = append(tokens, token{tokError, fmt.Sprintf("unexpected character %q", string(r))})
		i++
	}

	tokens = append(tokens, token{tokEOF, ""})
	return tokens
}

// Expr represents an AST node.
type Expr interface {
	Eval(env map[string]interface{}) (interface{}, error)
}

// LiteralExpr represents true, false, null, string, or number.
type LiteralExpr struct {
	Val interface{}
}

func (e *LiteralExpr) Eval(env map[string]interface{}) (interface{}, error) {
	return e.Val, nil
}

// VarExpr represents variables like auth, data, id.
type VarExpr struct {
	Name string
}

func (e *VarExpr) Eval(env map[string]interface{}) (interface{}, error) {
	val, ok := env[e.Name]
	if !ok {
		return nil, nil
	}
	return val, nil
}

// DotExpr represents attribute access like auth.uid.
type DotExpr struct {
	Left Expr
	Path string
}

func (e *DotExpr) Eval(env map[string]interface{}) (interface{}, error) {
	val, err := e.Left.Eval(env)
	if err != nil || val == nil {
		return nil, err
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	return m[e.Path], nil
}

// NotExpr represents logical negation (!expr).
type NotExpr struct {
	Sub Expr
}

func (e *NotExpr) Eval(env map[string]interface{}) (interface{}, error) {
	val, err := e.Sub.Eval(env)
	if err != nil {
		return nil, err
	}
	b, ok := val.(bool)
	if !ok {
		return false, nil
	}
	return !b, nil
}

// BinaryExpr represents operators: &&, ||, ==, !=.
type BinaryExpr struct {
	Op    tokenType
	Left  Expr
	Right Expr
}

func (e *BinaryExpr) Eval(env map[string]interface{}) (interface{}, error) {
	if e.Op == tokAnd {
		leftVal, err := e.Left.Eval(env)
		if err != nil {
			return nil, err
		}
		leftBool, _ := leftVal.(bool)
		if !leftBool {
			return false, nil
		}
		rightVal, err := e.Right.Eval(env)
		if err != nil {
			return nil, err
		}
		rightBool, _ := rightVal.(bool)
		return rightBool, nil
	}

	if e.Op == tokOr {
		leftVal, err := e.Left.Eval(env)
		if err != nil {
			return nil, err
		}
		leftBool, _ := leftVal.(bool)
		if leftBool {
			return true, nil
		}
		rightVal, err := e.Right.Eval(env)
		if err != nil {
			return nil, err
		}
		rightBool, _ := rightVal.(bool)
		return rightBool, nil
	}

	leftVal, err := e.Left.Eval(env)
	if err != nil {
		return nil, err
	}
	rightVal, err := e.Right.Eval(env)
	if err != nil {
		return nil, err
	}

	stringify := func(v interface{}) string {
		if v == nil {
			return "null"
		}
		return fmt.Sprintf("%v", v)
	}

	switch e.Op {
	case tokEq:
		if leftVal == nil && rightVal == nil {
			return true, nil
		}
		if leftVal == nil || rightVal == nil {
			return false, nil
		}
		return stringify(leftVal) == stringify(rightVal), nil
	case tokNe:
		if leftVal == nil && rightVal == nil {
			return false, nil
		}
		if leftVal == nil || rightVal == nil {
			return true, nil
		}
		return stringify(leftVal) != stringify(rightVal), nil
	}

	return nil, fmt.Errorf("unknown operator %v", e.Op)
}

type parser struct {
	tokens []token
	pos    int
}

// ParseRulesExpr parses tokenized rules into an Expr AST.
func ParseRulesExpr(tokens []token) (Expr, error) {
	p := &parser{tokens: tokens, pos: 0}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().typ != tokEOF {
		return nil, fmt.Errorf("unexpected token %q at end of expression", p.peek().val)
	}
	return expr, nil
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{tokEOF, ""}
	}
	return p.tokens[p.pos]
}

func (p *parser) next() token {
	t := p.peek()
	p.pos++
	return t
}

func (p *parser) match(typ tokenType) bool {
	if p.peek().typ == typ {
		p.pos++
		return true
	}
	return false
}

func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.match(tokOr) {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: tokOr, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expr, error) {
	left, err := p.parseEq()
	if err != nil {
		return nil, err
	}
	for p.match(tokAnd) {
		right, err := p.parseEq()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: tokAnd, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseEq() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		if p.peek().typ == tokEq || p.peek().typ == tokNe {
			op := p.next().typ
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *parser) parseUnary() (Expr, error) {
	if p.match(tokNot) {
		sub, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &NotExpr{Sub: sub}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.peek()
	var expr Expr

	switch t.typ {
	case tokTrue:
		p.next()
		expr = &LiteralExpr{Val: true}
	case tokFalse:
		p.next()
		expr = &LiteralExpr{Val: false}
	case tokNull:
		p.next()
		expr = &LiteralExpr{Val: nil}
	case tokString:
		p.next()
		expr = &LiteralExpr{Val: t.val}
	case tokNumber:
		p.next()
		f, _ := strconv.ParseFloat(t.val, 64)
		expr = &LiteralExpr{Val: f}
	case tokIdent:
		p.next()
		expr = &VarExpr{Name: t.val}
	case tokLParen:
		p.next()
		sub, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.match(tokRParen) {
			return nil, fmt.Errorf("expected closing parenthesis, got %q", p.peek().val)
		}
		expr = sub
	default:
		return nil, fmt.Errorf("expected expression, got %q", t.val)
	}

	for p.match(tokDot) {
		fieldTok := p.next()
		if fieldTok.typ != tokIdent {
			return nil, fmt.Errorf("expected field name after '.', got %q", fieldTok.val)
		}
		expr = &DotExpr{Left: expr, Path: fieldTok.val}
	}

	return expr, nil
}

// EvaluateRule parses and evaluates the given rule expression against the environment map.
func EvaluateRule(ruleExpr string, env map[string]interface{}) (bool, error) {
	trimmed := strings.TrimSpace(ruleExpr)
	if trimmed == "" {
		return false, nil
	}
	tokens := Tokenize(trimmed)
	expr, err := ParseRulesExpr(tokens)
	if err != nil {
		return false, fmt.Errorf("parse rule %q: %w", ruleExpr, err)
	}
	val, err := expr.Eval(env)
	if err != nil {
		return false, fmt.Errorf("evaluate rule %q: %w", ruleExpr, err)
	}
	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("rule expression %q did not evaluate to a boolean", ruleExpr)
	}
	return b, nil
}
