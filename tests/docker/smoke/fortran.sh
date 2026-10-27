# Smoke test for fortran: build and run a hello world with gfortran in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
cat > hello.f90 <<'EOF'
program hello
  implicit none
  integer :: i, total
  total = 0
  do i = 1, 9
    total = total + i
  end do
  print '(i0)', total
end program hello
EOF
gfortran -Wall -Werror -o hello hello.f90
_out="$(./hello)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"
cmake --version | head -1
