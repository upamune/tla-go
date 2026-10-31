---- MODULE M ----
EXTENDS H1, TLC, FiniteSets
I == INSTANCE H3 WITH C <- 2
VARIABLE x
Init == x = 0
Next == x' = Inc(x)
Inv == x < I!Lim
====
