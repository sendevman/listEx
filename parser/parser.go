package parser

import (
  "fmt"
  "github.com/kedebug/LispEx/ast"
  "github.com/kedebug/LispEx/lexer"
)

func ParseFromString(name, program string) ast.Node {
  return Parse(lexer.NewLexer(name, program))
}

func Parse(l *lexer.Lexer) ast.Node {
  elements := parser(l, make([]ast.Node, 0), " ")
  return ParseBlock(ast.NewTuple(elements))
}

func parser(l *lexer.Lexer, elements []ast.Node, delimiter string) []ast.Node {
  for token := l.NextToken(); token.Type != lexer.TokenEOF; token = l.NextToken() {
    switch token.Type {
    case lexer.TokenIdentifier:
      elements = append(elements, ast.NewName(token.Value))

    case lexer.TokenIntegerLiteral:
      elements = append(elements, ast.NewInt(token.Value))
    case lexer.TokenFloatLiteral:
      elements = append(elements, ast.NewFloat(token.Value))

    case lexer.TokenOpenParen:
      tuple := ast.NewTuple(parser(l, make([]ast.Node, 0), "("))
      elements = append(elements, tuple)
    case lexer.TokenCloseParen:
      if delimiter != "(" {
        panic(fmt.Sprint("read: unexpected `)'"))
      }
      return elements

    case lexer.TokenQuote:
      quote := []ast.Node{ast.NewName("quote")}
      quote = append(quote, parser(l, make([]ast.Node, 0), "'")...)
      elements = append(elements, ast.NewTuple(quote))
    case lexer.TokenQuasiquote:
      quasiquote := []ast.Node{ast.NewName("quasiquote")}
      quasiquote = append(quasiquote, parser(l, make([]ast.Node, 0), "`")...)
      elements = append(elements, ast.NewTuple(quasiquote))
    case lexer.TokenUnquote:
      unquote := []ast.Node{ast.NewName("unquote")}
      unquote = append(unquote, parser(l, make([]ast.Node, 0), ",")...)
      elements = append(elements, ast.NewTuple(unquote))
    case lexer.TokenUnquoteSplicing:
      unquoteSplicing := []ast.Node{ast.NewName("unquote-splicing")}
      unquoteSplicing = append(unquoteSplicing, parser(l, make([]ast.Node, 0), ",@")...)
      elements = append(elements, ast.NewTuple(unquoteSplicing))

    case lexer.TokenError:
      panic(fmt.Errorf("token error: %s", token.Value))
    default:
      panic(fmt.Errorf("unexpected token type: %v", token.Type))
    }

    if delimiter == "'" || delimiter == "`" || delimiter == "," || delimiter == ",@" {
      return elements
    }
  }
  if delimiter != " " {
    panic(fmt.Errorf("unclosed delimeter, expected: `%s'", delimiter))
  }
  return elements
}

func ParseNode(node ast.Node) ast.Node {
  tuple, ok := node.(*ast.Tuple)
  if !ok {
    return node
  }
  elements := tuple.Elements
  if len(elements) == 0 {
    panic(fmt.Errorf("syntax error, empty list"))
  }
  switch elements[0].(type) {
  case *ast.Name:
    name := elements[0].(*ast.Name)
    switch name.Identifier {
    case "seq":
      return ParseBlock(tuple)
    case "quote":
      return ParseQuote(tuple)
    case "quasiquote":
      // parse "unquote" and "unquote-splicing" in "quasiquote"
      // so they never go through ParseNode
      return ParseQuasiquote(tuple)
    case "unquote":
      return ParseUnquote(tuple)
      //panic(fmt.Sprint("unquote: not in quasiquote"))
    case "unquote-splicing":
      return ParseUnquoteSplicing(tuple)
      //panic(fmt.Sprint("unquote-splicing: not in quasiquote"))
    case "define":
      return ParseDefine(tuple)
    case "lambda":
      return ParseLambda(tuple)
    default:
      return ParseCall(tuple)
    }
  case *ast.Tuple:
    //(1). currying
    //  ((foo <arguments>) <arguments>)
    //(2). lambda
    //  ((lambda <formals> <body>) <arguments>)
    return ParseCall(tuple)
  default:
    panic(fmt.Sprintf("%s: not a procedure", tuple))
  }
}

func ParseBlock(tuple *ast.Tuple) *ast.Block {
  elements := tuple.Elements
  exprs := ParseList(elements)
  return ast.NewBlock(exprs)
}

func ParseQuote(tuple *ast.Tuple) *ast.Quote {
  // (quote <datum>)
  // '<datum>

  elements := tuple.Elements
  if len(elements) != 2 {
    panic(fmt.Sprint("quote: wrong number of parts"))
  }
  switch elements[1].(type) {
  case *ast.Tuple:
    slice := elements[1].(*ast.Tuple).Elements
    return ast.NewQuote(ExpandList(slice))
  default:
    return ast.NewQuote(elements[1])
  }
}

func ParseQuasiquote(tuple *ast.Tuple) *ast.Quasiquote {
  // (quasiquote <qq template>)
  // `<qq template>

  elements := tuple.Elements
  if len(elements) != 2 {
    panic(fmt.Sprint("quasiquote: wrong number of parts"))
  }
  switch elements[1].(type) {
  case *ast.Tuple:
    // `(1 ,(+ 1 1) ,@(cdr '(2 3 4)))
    // `((,(+ 2 3) c) d)
    qqt := elements[1].(*ast.Tuple).Elements
    slice := make([]ast.Node, 0, len(qqt))
    for _, node := range qqt {
      slice = append(slice, ParseQuasiquotedNode(node))
    }
    return ast.NewQuasiquote(ExpandList(slice))
  default:
    return ast.NewQuasiquote(elements[1])
  }
}

func ParseQuasiquotedNode(node ast.Node) ast.Node {
  if _, ok := node.(*ast.Tuple); !ok {
    return node
  }
  tuple := node.(*ast.Tuple)
  elements := tuple.Elements
  if len(elements) == 0 {
    return ast.NilPair
  }
  if name, ok := elements[0].(*ast.Name); ok {
    if name.Identifier == "unquote" {
      return ParseUnquote(tuple)
    } else if name.Identifier == "unquote-splicing" {
      return ParseUnquoteSplicing(tuple)
    } else if name.Identifier == "quasiquote" {
      return node
    }
  }
  slice := make([]ast.Node, 0, len(elements))
  for _, node := range elements {
    slice = append(slice, ParseQuasiquotedNode(node))
  }
  return ast.NewTuple(slice)
}

func ParseUnquote(tuple *ast.Tuple) *ast.Unquote {
  elements := tuple.Elements
  if len(elements) != 2 {
    panic(fmt.Sprint("unquote: wrong number of parts"))
  }
  return ast.NewUnquote(ParseNode(elements[1]))
}

func ParseUnquoteSplicing(tuple *ast.Tuple) *ast.UnquoteSplicing {
  elements := tuple.Elements
  if len(elements) != 2 {
    panic(fmt.Sprint("unquote-splicing: wrong number of parts"))
  }
  _, istuple := elements[1].(*ast.Tuple)
  return ast.NewUnquoteSplicing(ParseNode(elements[1]), !istuple)
}

func ParseDefine(tuple *ast.Tuple) *ast.Define {
  elements := tuple.Elements
  if len(elements) < 3 {
    panic(fmt.Sprint("define: bad syntax (missing expressions) ", tuple))
  }

  switch elements[1].(type) {
  case *ast.Name:
    // (define <variable> <expression>)
    if len(elements) > 3 {
      panic(fmt.Sprint("define: bad syntax (multiple expressions) ", tuple))
    }
    pattern := elements[1].(*ast.Name)
    value := ParseNode(elements[2])
    return ast.NewDefine(pattern, value)

  case *ast.Tuple:
    // (define (<variable> <formals>) <body>)
    // (define (<variable> . <formal>) <body>)
    tail := ast.NewBlock(ParseList(elements[2:]))
    function := ParseFunction(elements[1].(*ast.Tuple), tail)
    return ast.NewDefine(function.Caller, function)

  default:
    panic(fmt.Sprint("unsupported parser type ", elements[1]))
  }
}

func ParseFunction(tuple *ast.Tuple, tail ast.Node) *ast.Function {
  //  expand definition: e.g.
  //  ((f x) y) <body> =>
  //  (lambda (x)
  //    (lambda (y) <body>))

  lambda := ast.NewLambda(nil, tail)
  for {
    elements := tuple.Elements
    lambda.Params = ExpandFormals(elements[1:])

    // len(elements) must be greater than 0
    switch elements[0].(type) {
    case *ast.Name:
      return ast.NewFunction(elements[0].(*ast.Name), lambda)
    case *ast.Tuple:
      tuple = elements[0].(*ast.Tuple)
      lambda = ast.NewLambda(nil, lambda)
    default:
      panic(fmt.Sprint("unsupported parser type ", elements[0]))
    }
  }
}

func ParseCall(tuple *ast.Tuple) *ast.Call {
  elements := tuple.Elements
  if len(elements) == 0 {
    panic(fmt.Sprint("missing procedure expression"))
  }
  callee := ParseNode(elements[0])
  args := ParseList(elements[1:])
  return ast.NewCall(callee, args)
}

func ParseLambda(tuple *ast.Tuple) *ast.Lambda {
  // (lambda <formals> <body>)
  // switch <formals>:
  //  (<variable1> ...)
  //  <variable>
  //  (<variable1> ... <variablen> . <variablen+1>)

  elements := tuple.Elements
  if len(elements) < 3 {
    panic(fmt.Sprint("lambda: bad syntax: ", tuple))
  }
  pattern := elements[1]
  body := ast.NewBlock(ParseList(elements[2:]))

  switch pattern.(type) {
  case *ast.Name:
    return ast.NewLambda(pattern, body)
  case *ast.Tuple:
    formals := ExpandFormals(pattern.(*ast.Tuple).Elements)
    _, ok := formals.(*ast.Pair)
    if ok || formals == ast.NilPair {
      return ast.NewLambda(formals, body)
    } else {
      // (. <variable>) is not allowed
      panic(fmt.Sprint("lambda: illegal use of `.'"))
    }
  default:
    panic(fmt.Sprint("unsupported parser type ", pattern))
  }
}

func ParseList(nodes []ast.Node) []ast.Node {
  var parsed []ast.Node
  for _, node := range nodes {
    parsed = append(parsed, ParseNode(node))
  }
  return parsed
}
