---- MODULE MacroArgs ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
macro m(y) begin x := y; end macro
begin
l: m(1, 2);
end algorithm
*)
====
