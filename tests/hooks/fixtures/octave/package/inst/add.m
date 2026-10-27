## -*- texinfo -*-
## @deftypefn {} {@var{s} =} add (@var{a}, @var{b})
## Add two numbers.
## @end deftypefn
function s = add (a, b)
  s = a + b;
endfunction

%!assert (add (2, 3), 5)
%!test
%! assert (add (-1, 1), 0);
