;;; Fetch the dependencies of the systems the .asd files in the current
;;; directory define (their test systems' too) with Quicklisp, loaded
;;; before this file (asdf_hook in languages/_hooks.sh). The systems
;;; themselves aren't loaded here: the hook's build or test does that, and
;;; reports their errors. A dependency Quicklisp can't load is left for that
;;; step to report too.

(defun hecatron-dependency-name (spec)
  "A dependency spec's system name, or NIL for a feature that doesn't hold."
  (cond ((or (stringp spec) (symbolp spec)) (asdf:coerce-name spec))
        ((and (consp spec) (eq (first spec) :version)) (hecatron-dependency-name (second spec)))
        ((and (consp spec) (eq (first spec) :feature))
         (when (uiop:featurep (second spec)) (hecatron-dependency-name (third spec))))
        ((and (consp spec) (eq (first spec) :require)) nil)))

(let ((here (uiop:getcwd))
      (local '())
      (wanted '()))
  (dolist (asd (directory (merge-pathnames "*.asd" here)))
    (handler-case (asdf:load-asd asd)
      (error (e) (format *error-output* "~&hecatron: reading ~a: ~a~%" (file-namestring asd) e))))
  (dolist (name (asdf:registered-systems))
    (let* ((system (asdf:find-system name nil))
           (file (and system (asdf:system-source-file system))))
      (when (and file (uiop:pathname-equal (uiop:pathname-directory-pathname file) here))
        (push name local)
        (dolist (spec (append (asdf:system-defsystem-depends-on system) (asdf:system-depends-on system)))
          (let ((dependency (hecatron-dependency-name spec)))
            (when dependency (pushnew dependency wanted :test #'string=)))))))
  (dolist (dependency (reverse wanted))
    (unless (member dependency local :test #'string=)
      (handler-case (funcall (intern "QUICKLOAD" "QL") dependency :silent t)
        (error (e) (format *error-output* "~&hecatron: Quicklisp couldn't load ~a: ~a~%" dependency e))))))
