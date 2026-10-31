---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == /\ x < 2
        /\ Assert(x < 1, "boom")
        /\ x' = x + 1
====
