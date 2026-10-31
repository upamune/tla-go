---- MODULE M ----
EXTENDS Naturals
VARIABLES x, big
Init == x = 0 /\ big = [i \in 1..8 |-> [id |-> i, name |-> "abcdefgh", set |-> {i, i + 1}]]
Next == x < 2 /\ x' = x + 1 /\ big' = [big EXCEPT ![1].name = "z"]
Inv == x < 2
====
