---- MODULE SpecTE ----
EXTENDS Toolbox, TLC

\* 
\*  SpecTE follows
\* 

CONSTANTS Procs, p1, p2
VARIABLE owner
Init == owner = p1
Next == owner' \in Procs
Inv == owner = p1

CONSTANTS p1, p2

\* TRACE Sub-Action definition 0
Next_sa_0 ==
    (
        /\ owner = (
                p1
            )
        /\ owner' = (
                p2
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
([owner |-> p1]),
([owner |-> p2])
>>
----


\* VARIABLE TraceExp

\* TRACE INIT definition traceExploreInit
_SpecTEInit ==
    /\ owner = (
            p1
        )
\*     /\ TraceExp = TRUE

----

\* TRACE NEXT definition traceExploreNext
_SpecTENext ==
/\  \/ Next_sa_0
\* /\ TraceExp' = TraceExp



====
