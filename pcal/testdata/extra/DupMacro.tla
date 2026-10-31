---- MODULE DupMacro ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
macro m() begin x := 1; end macro
macro m() begin x := 2; end macro
begin
m();
end algorithm
*)
====
