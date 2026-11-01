---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..1
A == x < 0 /\ UNCHANGED x
B == x < 0 /\ UNCHANGED x
Next == A \/ B
H == x < 2
Inv == /\ H
       /\ x < 3
====
