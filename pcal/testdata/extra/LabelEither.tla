---- MODULE LabelEither ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: either x := 1; m: x := 2;
   or x := 3;
   or skip; n: x := 4; goto l;
   end either;
   x := 5;
   if x = 5 then o: x := 6; else x := 7; end if;
   q: x := 8;
end algorithm
*)
====
