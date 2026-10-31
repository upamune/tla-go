(* The leading comment is copied before the module. *)
---- MODULE M ----
EXTENDS Helper
VARIABLE x
Init == x = 0
Next == x' = Inc(x)
Inv == x < 2
====
