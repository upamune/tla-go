---- MODULE SpecTE ----
EXTENDS Sequences, Toolbox, TLC, Naturals, FiniteSets

\* 
\*  H4 follows
\* 

One == 1

\* 
\*  H3 follows
\* 

CONSTANT C
Lim == C * One

\* 
\*  H2 follows
\* 

K == 1

\* 
\*  H1 follows
\* 

Inc(n) == n + K

\* 
\*  SpecTE follows
\* 

I == INSTANCE H3 WITH C <- 2
VARIABLE x
Init == x = 0
Next == x' = Inc(x)
Inv == x < I!Lim
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
