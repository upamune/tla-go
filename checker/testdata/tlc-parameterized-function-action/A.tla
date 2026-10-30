---- MODULE A ----
EXTENDS Naturals
CONSTANT Limit
VARIABLE v
Step[i \in 1..2] == /\ v < Limit /\ v' = v + i
====
