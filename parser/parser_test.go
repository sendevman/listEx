package parser

import (
  "fmt"
  "github.com/kedebug/LispEx/scope"
  "testing"
)

func TestParser(t *testing.T) {
  var exprs string = `
    (define ((f x) . y) (print x) (print y))
    (lambda x body)
    (lambda (x) body)
    (lambda (x y . z) body)
    (lambda () body)
    ((lambda (x y) (+ x y)) 1 2)
  `
  block := ParseFromString("Parser", exprs)
  fmt.Println(block)
  env := scope.NewRootScope()
  fmt.Println(block.Eval(env))
}
