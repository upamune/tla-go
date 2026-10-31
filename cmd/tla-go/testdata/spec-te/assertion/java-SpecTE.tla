---- MODULE SpecTE ----
EXTENDS Toolbox, TLC, Naturals

\* 
\*  SpecTE follows
\* 

VARIABLE x
Init == x = 0
Next == /\ x < 2
        /\ Assert(x < 1, "boom")
        /\ x' = x + 1
\* TRACE Sub-Action definition 0
Next_sa_0 ==
    (
        /\ x = (
                0
            )
        /\ x' = (
                1
            )
    )

\* TRACE Action Constraint definition traceExploreActionConstraint
_SpecTEActionConstraint ==
<<
Next_sa_0
>>[TLCGet("level")]
----

def_ov_ID == 
<<
([x |-> 0]),
([x |-> 1])
>>
----


\* VARIABLE TraceExp

\* TRACE INIT definition traceExploreInit
_SpecTEInit ==
    /\ x = (
            0
        )
\*     /\ TraceExp = TRUE

----

\* TRACE NEXT definition traceExploreNext
_SpecTENext ==
/\  \/ Next_sa_0
\* /\ TraceExp' = TraceExp



====
