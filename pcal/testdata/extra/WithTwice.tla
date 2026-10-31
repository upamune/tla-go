---- MODULE WithTwice ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: with i \in 1..2 do
     x := i;
     x := i + 1;
   end with;
end algorithm
*)
====
