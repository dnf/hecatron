(ns add.core-test
  (:require [add.core :refer [add]]
            [clojure.test :refer [deftest is]]))

(deftest adds-two-numbers
  (is (= 5 (add 2 3))))
