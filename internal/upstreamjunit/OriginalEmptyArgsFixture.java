import java.io.File;
import java.util.Arrays;
import tlc2.TLC;

// Negative controls for the adapter's narrow EmptyTest argument contract.
public final class OriginalEmptyArgsFixture {
    public static void main(String[] args) throws Exception {
        String dump = "${metadir}" + File.separator + "tlc2.tool.EmptyTest.dot";
        String[] good = {"-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "-coverage", "1", "Empty"};
        TLC adapter = new TLC();
        if (!adapter.handleParameters(good)) { throw new AssertionError("valid arguments rejected"); }
        String[] wrongWorkers = good.clone();
        wrongWorkers[6] = "2";
        String[] wrongDump = good.clone();
        wrongDump[11] = "different.dot";
        for (String[] bad : new String[][] {wrongWorkers, wrongDump, Arrays.copyOf(good, good.length - 1)}) {
            try {
                adapter.handleParameters(bad);
                throw new AssertionError("unexpected arguments accepted");
            } catch (IllegalStateException expected) {
                // An unexpected argument must fail before Go is invoked.
            }
        }
    }
}
