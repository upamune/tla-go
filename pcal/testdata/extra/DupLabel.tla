---- MODULE DupLabel ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0, y = 1, x = 2;
begin
l: x := 1;
l: x := 2;
end algorithm
*)
====
