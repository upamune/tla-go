---- MODULE OptionsUnknown ----
EXTENDS Naturals, Sequences, TLC
(*
PlusCal options (wf bogus termination)
--algorithm a
variables x = 0;
begin
l: x := 1;
end algorithm
*)
====
