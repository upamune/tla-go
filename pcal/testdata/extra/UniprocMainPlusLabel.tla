---- MODULE UniprocMainPlusLabel ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
procedure p() begin p1: x := 1; return; end procedure
begin
m1:+ call p();
m2:- x := 2;
end algorithm
*)
====
