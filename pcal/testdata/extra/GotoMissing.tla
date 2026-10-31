---- MODULE GotoMissing ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: x := 1;
   goto nowhere;
end algorithm
*)
====
