(defpackage :words/tests
  (:use :cl)
  (:export #:run))
(in-package :words/tests)

(defun run ()
  "Signal an error unless every check holds; the test system's own
dependency (alexandria) comes from Quicklisp too."
  (assert (equal '("a" "b") (words:words " a  b ")))
  (assert (alexandria:length= 2 (words:words "a b")))
  (format t "~&words: 2 checks passed~%"))
