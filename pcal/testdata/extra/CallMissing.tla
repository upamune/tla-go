---- MODULE CallMissing ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
procedure p(a) begin l1: x := a; return; end procedure
begin
l: call q(1);
end algorithm
*)
====
