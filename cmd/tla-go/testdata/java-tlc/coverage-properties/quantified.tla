---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..1
A == x' = x
B == x < 0 /\ UNCHANGED x
Next == A \/ B
Inv == /\ x \in Nat
       /\ \A i \in 1..2 : i \in Nat
====
