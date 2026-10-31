---- MODULE NonAsciiComment ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0; (* résumé über 😀 *) y = 1;
begin
l: x := (* été *) <<1,
                          2>>;
end algorithm
*)
====
