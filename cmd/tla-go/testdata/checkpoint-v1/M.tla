---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == x' = x + 1 /\ x < 5
Inv == x < 5
====
