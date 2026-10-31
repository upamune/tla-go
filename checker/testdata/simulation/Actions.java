import tlc2.tool.Action;
import tlc2.tool.StateVec;
import tlc2.tool.TLCState;
import tlc2.tool.impl.FastTool;
import tlc2.util.FP64;
import util.SimpleFilenameToStream;
import util.UniqueString;

// Reports TLC's action order and successor order for the initial x = 0 state.
public final class Actions {
    public static void main(String[] args) {
        FP64.Init(0);
        FastTool tool = new FastTool("", "M", "M", new SimpleFilenameToStream());
        TLCState initial = tool.getInitStates().elementAt(0);
        for (Action action : tool.getActions()) {
            StateVec next = tool.getNextStates(action, initial);
            System.out.print("ACTION\t" + action.getName());
            for (int i = 0; i < next.size(); i++) {
                System.out.print("\t" + next.elementAt(i).lookup(UniqueString.uniqueStringOf("x")));
            }
            System.out.println();
        }
    }
}
