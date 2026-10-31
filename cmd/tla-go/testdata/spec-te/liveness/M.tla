---- MODULE M ----
EXTENDS Naturals, TLC, FiniteSets
VARIABLES x, y, z
vars == <<x, y, z>>
Init == x = 0 /\ y = 0 /\ z = <<1, 2>>
A == x' = x + 1 /\ UNCHANGED <<y, z>>
B == y' = y + 1 /\ z' = <<y, 2>> /\ UNCHANGED x
Next == A \/ B
Stop == x < 2 /\ Next
Cycle == x' = (x + 1) % 3 /\ UNCHANGED <<y, z>>
Live == Init /\ [][Cycle]_vars /\ WF_vars(Cycle)
Still == Init /\ [][Cycle]_vars
Inv4 == x + y < 3
Never == <>(x = 5)
====
