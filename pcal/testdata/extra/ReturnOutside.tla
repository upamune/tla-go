---- MODULE ReturnOutside ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
process p = 1
begin
l: x := 1;
   return;
end process
end algorithm
*)
====
