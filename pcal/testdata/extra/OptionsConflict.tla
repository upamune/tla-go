---- MODULE OptionsConflict ----
EXTENDS Naturals, Sequences, TLC
(*
PlusCal options (-wf, -sf)
--algorithm a
variables x = 0;
begin
l: x := 1;
end algorithm
*)
====
