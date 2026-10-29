(defpackage :words
  (:use :cl)
  (:export #:words))
(in-package :words)

(defun words (text)
  "The words in text, split at spaces."
  (split-sequence:split-sequence #\Space text :remove-empty-subseqs t))
