---- MODULE UndefMacro ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
macro m(y) begin x := y; end macro
begin
l: n(1);
end algorithm
*)
====
