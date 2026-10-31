---- MODULE CallArgs ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
procedure p(a) begin l1: x := a; return; end procedure
begin
l: call p(1, 2);
end algorithm
*)
====
