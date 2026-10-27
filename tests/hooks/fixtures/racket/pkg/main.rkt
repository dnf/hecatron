#lang racket/base
;; Adds two numbers.
(provide add)

(define (add a b)
  (+ a b))

(module+ test
  (require rackunit)
  (check-equal? (add 2 3) 5))
