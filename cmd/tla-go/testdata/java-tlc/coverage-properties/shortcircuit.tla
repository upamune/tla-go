---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..1
A == x' = x
B == x < 0 /\ UNCHANGED x
Next == A \/ B
Inv == /\ x \in Nat
       /\ (x = 0 \/ x = 1)
       /\ \A i \in 2..1 : i \in Nat
====
