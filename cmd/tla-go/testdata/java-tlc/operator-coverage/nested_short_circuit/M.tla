---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..1
A == x' = 0
B == x' = 1
Next == A \/ B
Inv == /\ (x = 0 \/ (x = 1 /\ x < 2))
       /\ (x < 2 \/ x = 0)
====
