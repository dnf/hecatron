(ns build
  (:require [clojure.tools.build.api :as b]))

(def lib 'example/add)
(def version "0.1.0")
(def class-dir "target/classes")
(def jar-file (format "target/%s-%s.jar" (name lib) version))

(defn jar [_]
  (let [basis (b/create-basis {:project "deps.edn"})]
    (b/write-pom {:class-dir class-dir :lib lib :version version :basis basis :src-dirs ["src"]})
    (b/copy-dir {:src-dirs ["src"] :target-dir class-dir})
    (b/jar {:class-dir class-dir :jar-file jar-file})))
