---- MODULE FairProcesses ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0, flag = [i \in 1..3 |-> FALSE];
procedure inc(n = 1)
  variable tmp = 0;
begin
i1:+ tmp := x;
i2:- x := tmp + n;
i3: return;
end procedure;
procedure dbl()
begin
d1: call inc(x);
d2: return;
end procedure;
fair process p \in 1..2
  variables loc \in {1, 2}, other = self;
begin
p1:+ flag[self] := TRUE;
p2:- call dbl();
p3: call inc(2);
p4: skip;
end process;
fair+ process q = 3
  variable w = 0;
begin
q1:- w := w + 1;
q2: call inc(w) ;
q3: goto q1;
end process;
process r \in {4, 5}
begin
r1: await x > 2;
    print <<"r", self>>;
    assert x > 0;
end process
end algorithm
*)
====
