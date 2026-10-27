(defpackage :add/tests
  (:use :cl)
  (:export #:run))
(in-package :add/tests)

(defun run ()
  "Signal an error unless every check holds."
  (assert (= 5 (add:add 2 3)))
  (format t "~&add: 1 check passed~%"))
