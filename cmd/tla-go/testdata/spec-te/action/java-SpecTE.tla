---- MODULE SpecTE ----
EXTENDS Toolbox, Naturals, TLC

\* 
\*  SpecTE follows
\* 

VARIABLES x, y
Init == x = 0 /\ y = 0
Next == x' = x + 1 /\ y' = y
Act == [][x' < 3]_<<x, y>>
\* TRACE Sub-Action definition 0
Next_sa_0 ==
    (
        /\ x = (
                0
            )
        /\ y = (
                0
            )
        /\ x' = (
                1
            )
        /\ y' = (
                0
            )
    )

\* TRACE Sub-Action definition 1
Next_sa_1 ==
    (
        /\ x = (
                1
            )
        /\ y = (
                0
            )
        /\ x' = (
                2
            )
        /\ y' = (
                0
            )
    )

\* TRACE Sub-Action definition 2
Next_sa_2 ==
    (
        /\ x = (
                2
            )
        /\ y = (
                0
            )
        /\ x' = (
                3
            )
        /\ y' = (
                0
            )
    )

\* TRACE Action Constraint definition traceExploreActionConstraint
_SpecTEActionConstraint ==
<<
Next_sa_0,
Next_sa_1,
Next_sa_2
>>[TLCGet("level")]
----

def_ov_ID == 
<<
([x |-> 0,y |-> 0]),
([x |-> 1,y |-> 0]),
([x |-> 2,y |-> 0]),
([x |-> 3,y |-> 0])
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
\*     /\ TraceExp = TRUE

----

\* TRACE NEXT definition traceExploreNext
_SpecTENext ==
/\  \/ Next_sa_0
    \/ Next_sa_1
    \/ Next_sa_2
\* /\ TraceExp' = TraceExp



====
