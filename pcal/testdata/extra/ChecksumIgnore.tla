---- MODULE ChecksumIgnore ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
begin
l: x := 1;
end algorithm
*)
\* BEGIN TRANSLATION (ChkSum(PlusCal) \in STRING /\ chksum(tla+) = "12")
\* END TRANSLATION
(*
*)
====
