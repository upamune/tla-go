---- MODULE DivergenceTest_divergenceTest02 ----

(*
--algorithm a
begin
  print "msg";
end algorithm; *)
\* BEGIN TRANSLATION (chksum(PCal) = "4860ac97" /\ chksum(TLA+) = "af3d9146")
VARIABLE pc

vars == << pc >>

Init == /\ pc = "Lbl_1"

Lbl_1 == /\ pc = "Lbl_1"
         /\ TRUE
         /\ pc' = "Done"

(* Allow infinite stuttering to prevent deadlock on termination. *)
Terminating == pc = "Done" /\ UNCHANGED vars

Next == Lbl_1
           \/ Terminating

Spec == Init /\ [][Next]_vars

Termination == <>(pc = "Done")

\* END TRANSLATION

=========================