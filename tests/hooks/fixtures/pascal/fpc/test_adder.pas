program TestAdder;

uses Adder;

begin
  if Add(2, 3) <> 5 then
  begin
    WriteLn(StdErr, '2 + 3 <> 5');
    Halt(1);
  end;
  WriteLn('ok');
end.
