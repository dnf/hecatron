:- module(add, [add/3]).

%!  add(+A, +B, -Sum) is det.
%
%   Sum is A plus B.
add(A, B, Sum) :-
    Sum is A + B.
