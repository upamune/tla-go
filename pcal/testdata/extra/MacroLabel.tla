---- MODULE MacroLabel ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
macro m() begin l: x := 1; end macro
begin
m();
end algorithm
*)
====
