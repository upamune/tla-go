---- MODULE EmptyIf ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: if TRUE then
   end if;
end algorithm
*)
====
