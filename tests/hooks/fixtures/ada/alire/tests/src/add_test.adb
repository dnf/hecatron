with Ada.Command_Line;
with Ada.Text_IO;
with Adder;

procedure Add_Test is
begin
   if Adder.Add (2, 3) /= 5 then
      Ada.Text_IO.Put_Line (Ada.Text_IO.Standard_Error, "2 + 3 /= 5");
      Ada.Command_Line.Set_Exit_Status (Ada.Command_Line.Failure);
   else
      Ada.Text_IO.Put_Line ("ok");
   end if;
end Add_Test;
