---- MODULE M ----
EXTENDS Naturals
VARIABLES x, y
Init == x = 0 /\ y = 0
Next == x' = x + 1 /\ y' = y
Act == [][x' < 3]_<<x, y>>
====
