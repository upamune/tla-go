---- MODULE TextAfterComment ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: x := 1;
end algorithm *) Foo == 1
(*
*)
====
