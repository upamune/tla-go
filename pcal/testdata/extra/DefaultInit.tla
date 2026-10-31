---- MODULE DefaultInit ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x, y = 1;
procedure p(a, b = 2) variable c; begin l: c := a; k: return; end procedure
process q \in 1..2 variable d; begin m: call p(1, d); end process
end algorithm
*)
====
