(* The leading comment is copied before the module. *)
---- MODULE SpecTE ----
EXTENDS Helper, Toolbox, Naturals, TLC

\* 
\*  SpecTE follows
\* 

VARIABLE x
Init == x = 0
Next == x' = Inc(x)
Inv == x < 2
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

\* TRACE Sub-Action definition 1
Next_sa_1 ==
    (
        /\ x = (
                1
            )
        /\ x' = (
                2
            )
    )

\* TRACE Action Constraint definition traceExploreActionConstraint
_SpecTEActionConstraint ==
<<
Next_sa_0,
Next_sa_1
>>[TLCGet("level")]
----

def_ov_ID == 
<<
([x |-> 0]),
([x |-> 1]),
([x |-> 2])
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
    \/ Next_sa_1
\* /\ TraceExp' = TraceExp



====
