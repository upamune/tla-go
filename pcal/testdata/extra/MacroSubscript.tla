---- MODULE MacroSubscript ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = [i \in 1..3 |-> 0];
macro m(y, v) begin y[2] := v; y[3] := v + 1 || y[1] := 7; end macro
begin
l: m(x, 4);
end algorithm
*)
====
