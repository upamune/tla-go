---- MODULE WhileInWith ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: with i \in 1..2 do
     while x < 3 do x := x + 1; end while;
   end with;
end algorithm
*)
====
