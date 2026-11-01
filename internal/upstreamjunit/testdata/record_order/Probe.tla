---- MODULE Probe ----
EXTENDS TLC, FiniteSets
CONSTANTS A, B
VARIABLE x
S == [op : {A, B}, adr : {A, B}]
Sym == Permutations({A, B})
Init == /\ x \in S
        /\ Print("direct", x) = x
        /\ Print("string", ToString(x)) = ToString(x)
        /\ Print("nested", <<x, [i \in {1} |-> x]>>) = <<x, [i \in {1} |-> x]>>
        /\ x = [adr |-> x.adr, op |-> x.op]
Next == x' = x
Alias == [shared |-> <<x, x>>, literal |-> [adr |-> x.adr, op |-> x.op]]
View == x
====
