# Simulation oracle

`Actions.java` and `RandomWalk.java` are small test harnesses for the Java TLC
1.7.4 implementation at TLA+ Tools commit
`5a47802b5c391f59ecdd44117981f4ff8c0656ba`. They call `FastTool.getActions`,
`FastTool.getNextStates`, and `SimulationWorker.randomNextStates` directly;
they are not implementations of TLC and are only compiled during optional
Java differential tests when `TLC_JAR` is set. The random-walk test also has a
recorded 1.7.4 result so it can run without Java.
