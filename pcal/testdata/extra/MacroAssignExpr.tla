---- MODULE MacroAssignExpr ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
macro m(y) begin y := 1; end macro
begin
l: m(x + 1);
end algorithm
*)
====
