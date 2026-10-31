---- MODULE Renamed ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
procedure p(x1) variable y = 0; begin p1: y := x1; p2: return; end procedure
procedure q(y) variable x2 = 0; begin p1: x2 := y; p2: return; end procedure
process r = 1 variable y = 0; begin p1: call p(1); p2: call q(2); end process
end algorithm
*)
====
