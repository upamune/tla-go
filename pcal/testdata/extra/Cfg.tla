---- MODULE Cfg ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a {
  variables x = 0, s = {};
  define { Inv == x >= 0  Twice(v) == 2 * v }
  macro add(v) { s := s \cup {v} }
  procedure p(a = 1) { pp: x := Twice(a); return }
  process (w \in 1..2) variables t = 0; {
    w1: while (x < 3) {
      either { x := x + 1; add(self) } or { t := t + 1 } or { call p(self) } ;
      w2: with (i \in 1..2, j = i + 1) { x := x + j }
    };
    w3: if (x > 5) { x := 0 } else if (x = 4) { x := 1 } else { skip }
  }
  process (z = 9) { z1: await x > 0; z2: print x }
}
*)
====
