---- MODULE NonBmpEscape ----
EXTENDS Naturals, TLC
(*
--algorithm a
variables x = 0;
begin
l: x := "\😀";
end algorithm
*)
====
