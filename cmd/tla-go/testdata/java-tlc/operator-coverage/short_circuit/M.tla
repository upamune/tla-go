---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..1
A == x < 0 /\ UNCHANGED x
B == x < 0 /\ UNCHANGED x
Next == A \/ B
F(n) == \A i \in 1..n : i < 3
Inv == \/ x < 2
       \/ F(2)
====
