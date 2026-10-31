---- MODULE CallReturnGoto ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables x = 0;
procedure f(n)
  variable k = n;
begin
f1: if n > 0 then
      call g(n - 1);
      return;
    else
      call f(n - 1) ;
      return;
    end if;
end procedure;
procedure g(m)
begin
g1: x := x + m;
    call f(m) ;
    goto g2;
g2: return;
end procedure;
begin
m1: call f(3);
m2: call g(1);
    goto m1;
end algorithm
*)
====
