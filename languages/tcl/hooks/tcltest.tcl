# Run the .test files in a directory (the first argument) with tcltest, in
# this process, and exit non-zero when any test fails: a test file's own
# cleanupTests exits zero either way.
package require tcltest 2.5
tcltest::configure -testdir [file normalize [lindex $argv 0]] -singleproc 1
exit [tcltest::runAllTests]
