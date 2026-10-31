import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.atomic.LongAdder;
import tlc2.tool.SimulationWorker;
import tlc2.tool.StateVec;
import tlc2.tool.TLCState;
import tlc2.tool.impl.FastTool;
import tlc2.util.FP64;
import tlc2.util.RandomGenerator;
import util.SimpleFilenameToStream;
import util.UniqueString;

// Exercise the same methods as a single TLC simulation worker.
public final class RandomWalk {
    public static void main(String[] args) {
        FP64.Init(0);
        FastTool tool = new FastTool("", "M", "M", new SimpleFilenameToStream());
        long seed = Long.parseLong(args[0]);
        RandomGenerator master = new RandomGenerator(seed);
        RandomGenerator random = new RandomGenerator(master.nextLong());
        SimulationWorker worker = new SimulationWorker(0, tool, new LinkedBlockingQueue<>(),
            0, 20, 1, true, null, null, new LongAdder(), new LongAdder());
        StateVec initial = tool.getInitStates();
        TLCState state = initial.elementAt((int) Math.floor(random.nextDouble() * initial.size()));
        for (int step = 0; step < 20; step++) {
            StateVec states = worker.randomNextStates(random, state);
            if (states == null) {
                System.out.println("WALK\tdeadlock");
                break;
            }
            System.out.print("WALK");
            for (int i = 0; i < states.size(); i++) {
                System.out.print("\t" + states.elementAt(i).lookup(UniqueString.uniqueStringOf("x")));
            }
            state = states.elementAt((int) Math.floor(random.nextDouble() * states.size()));
            System.out.println("\t=>\t" + state.lookup(UniqueString.uniqueStringOf("x")));
        }
    }
}
