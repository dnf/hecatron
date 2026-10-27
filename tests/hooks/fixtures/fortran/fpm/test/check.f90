program add_test
    use add_mod, only: add
    implicit none
    if (add(2, 3) /= 5) error stop "2 + 3 /= 5"
    print '(a)', "ok"
end program add_test
