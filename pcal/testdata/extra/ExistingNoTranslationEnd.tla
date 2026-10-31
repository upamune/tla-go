---- MODULE ExistingNoTranslationEnd ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: x := 1;
end algorithm
*)
\* BEGIN TRANSLATION
VARIABLE x
(*
*)
====
