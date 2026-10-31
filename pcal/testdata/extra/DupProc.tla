---- MODULE DupProc ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
procedure p() begin l1: return; end procedure
procedure p() begin l2: return; end procedure
process q = 1 begin l: call p(); end process
process q = 2 begin m: skip; end process
end algorithm
*)
====
