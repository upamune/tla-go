---- MODULE EmptyThen ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: if x = 0 then else x := 1; end if;
end algorithm
*)
====
