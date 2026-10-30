---- MODULE M ----
EXTENDS Naturals
VARIABLE x
P(k) == INSTANCE A WITH Limit <- k, v <- x
Init == x = 0
Next == \E i \in 1..2 : P(3)!Step[i]
Inv == x < 3
====
