(defsystem "words"
  :description "Splits text into words, with a dependency from Quicklisp."
  :license "MIT"
  :depends-on ("split-sequence")
  :components ((:module "src" :components ((:file "words"))))
  :in-order-to ((test-op (test-op "words/tests"))))

(defsystem "words/tests"
  :depends-on ("words" "alexandria")
  :components ((:module "tests" :components ((:file "words"))))
  :perform (test-op (o c) (symbol-call :words/tests :run)))
