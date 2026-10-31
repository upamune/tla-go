---- MODULE M ----
EXTENDS TLC
CONSTANTS Procs, p1, p2
VARIABLE owner
Init == owner = p1
Next == owner' \in Procs
Inv == owner = p1
====
