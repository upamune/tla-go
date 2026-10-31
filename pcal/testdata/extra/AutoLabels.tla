---- MODULE AutoLabels ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0, y = 0;
procedure p() begin return; end procedure
begin
  x := 1;
  x := 2 || y := 3;
  with i \in 1..2 do y := i; end with;
  call p();
  while x < 5 do x := x + 1; end while;
  if x = 5 then goto l; end if;
l: y := x;
end algorithm
*)
====
