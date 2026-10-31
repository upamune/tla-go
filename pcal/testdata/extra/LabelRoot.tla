---- MODULE LabelRoot ----
EXTENDS Naturals, TLC
(*
--algorithm a
variables x = 0, y = 0, w = 0;
procedure p(a) begin
  x := a;
  y := y + 1;
  return;
end procedure
process q \in 1..2
begin
  x := 1;
  y := 2;
  call p(3);
  while x < 10 do
    x := x + 1;
    if x > 5 then y := y + x; else y := y - 1; end if;
  end while;
  either x := 7 or y := 8 end either;
  with z \in {1, 2} do w := z end with;
  print <<x, y>>;
end process
process r = 3
begin
  w := 4;
  await x > 0;
end process
end algorithm
*)
====
