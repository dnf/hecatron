let () =
  if Add.add 2 3 <> 5 then (
    prerr_endline "2 + 3 <> 5";
    exit 1);
  print_endline "ok"
