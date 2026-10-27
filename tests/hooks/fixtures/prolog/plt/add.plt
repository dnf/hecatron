:- use_module(add).

:- begin_tests(add).

test(adds_two_numbers) :-
    add(2, 3, 5).

:- end_tests(add).
