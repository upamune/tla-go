---- MODULE DivergenceTest_divergenceTest05 ----

(*
--algorithm a
begin
  print "msg";
end algorithm; *)
\* BEGIN TRANSLATION   (checksum(PlusCal) \in  STRING /\ ChkSum(tla+) \in STRING)  
VARIABLE pc

vars == << pc >>

Init == /\ pc = "Lbl_1"

Lbl_1 == /\ pc = "Lbl_1"
         /\ TRUE
         /\ pc' = "Done"

(* Allow infinite stuttering to prevent deadlock on termination. *)
Terminating == pc = "Done" /\ UNCHANGED vars

Next == Lbl_1

Spec == Init /\ [][Next]_vars

Termination == <>(pc = "Done")

\* END TRANSLATION

=========================