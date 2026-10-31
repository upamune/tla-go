---- MODULE NonBmpLexeme ----
EXTENDS Naturals, TLC
(*
--algorithm a
variables x = 0, y = 0;
begin
l: x := 1; (* 😀 *)
   y := 2 + 😀;
end algorithm
*)
====
