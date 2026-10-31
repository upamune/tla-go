---- MODULE SpecTE ----
EXTENDS Toolbox, TLC, Naturals, FiniteSets

\* 
\*  SpecTE follows
\* 

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
\* TRACE Sub-Action definition 0
Live_sa_0 ==
    (
        /\ x = (
                0
            )
        /\ y = (
                0
            )
        /\ z = (
                <<1, 2>>
            )
        /\ x' = (
                1
            )
        /\ y' = (
                0
            )
        /\ z' = (
                <<1, 2>>
            )
    )

\* TRACE Sub-Action definition 1
Live_sa_1 ==
    (
        /\ x = (
                1
            )
        /\ y = (
                0
            )
        /\ z = (
                <<1, 2>>
            )
        /\ x' = (
                2
            )
        /\ y' = (
                0
            )
        /\ z' = (
                <<1, 2>>
            )
    )

\* TRACE Sub-Action definition 2
Live_sa_2 ==
    (
        /\ x = (
                2
            )
        /\ y = (
                0
            )
        /\ z = (
                <<1, 2>>
            )
        /\ x' = (
                0
            )
        /\ y' = (
                0
            )
        /\ z' = (
                <<1, 2>>
            )
    )

\* TRACE Action Constraint definition traceExploreActionConstraint
_SpecTEActionConstraint ==
<<
Live_sa_0,
Live_sa_1,
Live_sa_2
>>[TLCGet("level")]
----

def_ov_ID == 
<<
([x |-> 0,y |-> 0,z |-> <<1, 2>>]),
([x |-> 1,y |-> 0,z |-> <<1, 2>>]),
([x |-> 2,y |-> 0,z |-> <<1, 2>>])
>>
----


\* VARIABLE TraceExp

\* TRACE INIT definition traceExploreInit
_SpecTEInit ==
    /\ x = (
            0
        )
    /\ y = (
            0
        )
    /\ z = (
            <<1, 2>>
        )
\*     /\ TraceExp = TRUE

----

\* TRACE NEXT definition traceExploreNext
_SpecTENext ==
/\  \/ Live_sa_0
    \/ Live_sa_1
    \/ Live_sa_2
\* /\ TraceExp' = TraceExp



====
