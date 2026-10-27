(defsystem "add"
  :description "Adds two numbers."
  :license "MIT"
  :components ((:module "src" :components ((:file "add"))))
  :in-order-to ((test-op (test-op "add/tests"))))

(defsystem "add/tests"
  :depends-on ("add")
  :components ((:module "tests" :components ((:file "add"))))
  :perform (test-op (o c) (symbol-call :add/tests :run)))
