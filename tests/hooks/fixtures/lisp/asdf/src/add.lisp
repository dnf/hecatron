(defpackage :add
  (:use :cl)
  (:export #:add))
(in-package :add)

(defun add (a b)
  "Add two numbers."
  (+ a b))
