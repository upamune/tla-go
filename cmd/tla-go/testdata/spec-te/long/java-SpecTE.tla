---- MODULE SpecTE ----
EXTENDS Toolbox, Naturals, TLC

\* 
\*  SpecTE follows
\* 

VARIABLES x, big
Init == x = 0 /\ big = [i \in 1..8 |-> [id |-> i, name |-> "abcdefgh", set |-> {i, i + 1}]]
Next == x < 2 /\ x' = x + 1 /\ big' = [big EXCEPT ![1].name = "z"]
Inv == x < 2
\* TRACE Sub-Action definition 0
Next_sa_0 ==
    (
        /\ x = (
                0
            )
        /\ big = (
                << [id |-> 1, name |-> "abcdefgh", set |-> {1, 2}],
                   [id |-> 2, name |-> "abcdefgh", set |-> {2, 3}],
                   [id |-> 3, name |-> "abcdefgh", set |-> {3, 4}],
                   [id |-> 4, name |-> "abcdefgh", set |-> {4, 5}],
                   [id |-> 5, name |-> "abcdefgh", set |-> {5, 6}],
                   [id |-> 6, name |-> "abcdefgh", set |-> {6, 7}],
                   [id |-> 7, name |-> "abcdefgh", set |-> {7, 8}],
                   [id |-> 8, name |-> "abcdefgh", set |-> {8, 9}] >>
            )
        /\ x' = (
                1
            )
        /\ big' = (
                << [id |-> 1, name |-> "z", set |-> {1, 2}],
                   [id |-> 2, name |-> "abcdefgh", set |-> {2, 3}],
                   [id |-> 3, name |-> "abcdefgh", set |-> {3, 4}],
                   [id |-> 4, name |-> "abcdefgh", set |-> {4, 5}],
                   [id |-> 5, name |-> "abcdefgh", set |-> {5, 6}],
                   [id |-> 6, name |-> "abcdefgh", set |-> {6, 7}],
                   [id |-> 7, name |-> "abcdefgh", set |-> {7, 8}],
                   [id |-> 8, name |-> "abcdefgh", set |-> {8, 9}] >>
            )
    )

\* TRACE Sub-Action definition 1
Next_sa_1 ==
    (
        /\ x = (
                1
            )
        /\ big = (
                << [id |-> 1, name |-> "z", set |-> {1, 2}],
                   [id |-> 2, name |-> "abcdefgh", set |-> {2, 3}],
                   [id |-> 3, name |-> "abcdefgh", set |-> {3, 4}],
                   [id |-> 4, name |-> "abcdefgh", set |-> {4, 5}],
                   [id |-> 5, name |-> "abcdefgh", set |-> {5, 6}],
                   [id |-> 6, name |-> "abcdefgh", set |-> {6, 7}],
                   [id |-> 7, name |-> "abcdefgh", set |-> {7, 8}],
                   [id |-> 8, name |-> "abcdefgh", set |-> {8, 9}] >>
            )
        /\ x' = (
                2
            )
        /\ big' = (
                << [id |-> 1, name |-> "z", set |-> {1, 2}],
                   [id |-> 2, name |-> "abcdefgh", set |-> {2, 3}],
                   [id |-> 3, name |-> "abcdefgh", set |-> {3, 4}],
                   [id |-> 4, name |-> "abcdefgh", set |-> {4, 5}],
                   [id |-> 5, name |-> "abcdefgh", set |-> {5, 6}],
                   [id |-> 6, name |-> "abcdefgh", set |-> {6, 7}],
                   [id |-> 7, name |-> "abcdefgh", set |-> {7, 8}],
                   [id |-> 8, name |-> "abcdefgh", set |-> {8, 9}] >>
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
([x |-> 0,big |-> << [id |-> 1, name |-> "abcdefgh", set |-> {1, 2}],
   [id |-> 2, name |-> "abcdefgh", set |-> {2, 3}],
   [id |-> 3, name |-> "abcdefgh", set |-> {3, 4}],
   [id |-> 4, name |-> "abcdefgh", set |-> {4, 5}],
   [id |-> 5, name |-> "abcdefgh", set |-> {5, 6}],
   [id |-> 6, name |-> "abcdefgh", set |-> {6, 7}],
   [id |-> 7, name |-> "abcdefgh", set |-> {7, 8}],
   [id |-> 8, name |-> "abcdefgh", set |-> {8, 9}] >>]),
([x |-> 1,big |-> << [id |-> 1, name |-> "z", set |-> {1, 2}],
   [id |-> 2, name |-> "abcdefgh", set |-> {2, 3}],
   [id |-> 3, name |-> "abcdefgh", set |-> {3, 4}],
   [id |-> 4, name |-> "abcdefgh", set |-> {4, 5}],
   [id |-> 5, name |-> "abcdefgh", set |-> {5, 6}],
   [id |-> 6, name |-> "abcdefgh", set |-> {6, 7}],
   [id |-> 7, name |-> "abcdefgh", set |-> {7, 8}],
   [id |-> 8, name |-> "abcdefgh", set |-> {8, 9}] >>]),
([x |-> 2,big |-> << [id |-> 1, name |-> "z", set |-> {1, 2}],
   [id |-> 2, name |-> "abcdefgh", set |-> {2, 3}],
   [id |-> 3, name |-> "abcdefgh", set |-> {3, 4}],
   [id |-> 4, name |-> "abcdefgh", set |-> {4, 5}],
   [id |-> 5, name |-> "abcdefgh", set |-> {5, 6}],
   [id |-> 6, name |-> "abcdefgh", set |-> {6, 7}],
   [id |-> 7, name |-> "abcdefgh", set |-> {7, 8}],
   [id |-> 8, name |-> "abcdefgh", set |-> {8, 9}] >>])
>>
----


\* VARIABLE TraceExp

\* TRACE INIT definition traceExploreInit
_SpecTEInit ==
    /\ x = (
            0
        )
    /\ big = (
            << [id |-> 1, name |-> "abcdefgh", set |-> {1, 2}],
               [id |-> 2, name |-> "abcdefgh", set |-> {2, 3}],
               [id |-> 3, name |-> "abcdefgh", set |-> {3, 4}],
               [id |-> 4, name |-> "abcdefgh", set |-> {4, 5}],
               [id |-> 5, name |-> "abcdefgh", set |-> {5, 6}],
               [id |-> 6, name |-> "abcdefgh", set |-> {6, 7}],
               [id |-> 7, name |-> "abcdefgh", set |-> {7, 8}],
               [id |-> 8, name |-> "abcdefgh", set |-> {8, 9}] >>
        )
\*     /\ TraceExp = TRUE

----

\* TRACE NEXT definition traceExploreNext
_SpecTENext ==
/\  \/ Next_sa_0
    \/ Next_sa_1
\* /\ TraceExp' = TraceExp



====
