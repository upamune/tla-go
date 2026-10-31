---- MODULE WithLabel ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: with i \in 1..2 do
     m: x := i;
   end with;
end algorithm
*)
====
