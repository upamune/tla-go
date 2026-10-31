---- MODULE NonAscii ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0; (* résumé über *) y = 1;
begin
l: x := (* été *) <<1,
                2>>;
   y := é;
end algorithm
*)
====
