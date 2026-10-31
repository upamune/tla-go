---- MODULE LongNames ----
EXTENDS Naturals, Sequences, TLC
(*
--algorithm a
variables averyveryverylongvariablenameone = 0, averyveryverylongvariablenametwo = 1,
          averyveryverylongvariablenamethree = 2, averyveryverylongvariablenamefour = 3;
process averyveryverylongprocessname \in {1, 2, 3}
  variables locala = 0, localb = 1, localc = 2, locald = 3, locale = 4, localf = 5;
begin
averyveryverylonglabelname1: averyveryverylongvariablenameone := 1;
averyveryverylonglabelname2: either locala := 1; or localb := 2; or localc := 3; end either;
averyveryverylonglabelname3: if locald = 1 then locale := 1; else localf := 2 || locala := 3; end if;
averyveryverylonglabelname4: assert averyveryverylongvariablenameone + averyveryverylongvariablenametwo + averyveryverylongvariablenamethree > 0;
end process
end algorithm
*)
====
