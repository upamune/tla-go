---- MODULE OptionsInFile ----
EXTENDS Naturals, Sequences, TLC
(*
PlusCalxx options (-sf) PlusCal optionsx (-sf) PlusCal options (sf, -label, labelRoot Q, version 1.5, noDoneDisjunct, -lineWidth 65)
--algorithm a
variables x = 0, y = <<1, 2, 3>>;
process p \in 1..2
variables z = self;
begin
  x := x + z;
  y[self] := 0;
end process
end algorithm
*)
====
