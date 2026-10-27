module add_mod
    implicit none
    private
    public :: add
contains
    pure integer function add(a, b)
        integer, intent(in) :: a, b
        add = a + b
    end function add
end module add_mod
