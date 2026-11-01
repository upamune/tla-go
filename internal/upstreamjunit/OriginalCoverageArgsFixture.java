import java.io.File;
import java.util.Arrays;
import tlc2.TLC;

// Prove that only the original argument lists can reach the Go helper.
public final class OriginalCoverageArgsFixture {
    public static void main(String[] args) throws Exception {
        for (String target : new String[] {"B", "G"}) {
            System.setProperty("tla.coverage.target", target);
            String dump = "${metadir}" + File.separator + "tlc2.tool.coverage." + target + "CoverageTest.dot";
            String[] good = {"-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "-coverage", "9999", target};
            TLC adapter = new TLC();
            if (!adapter.handleParameters(good)) { throw new AssertionError("valid arguments rejected"); }
            String[] wrongWorkers = good.clone();
            wrongWorkers[6] = "2";
            String[] wrongDump = good.clone();
            wrongDump[11] = "different.dot";
            String[] wrongSpec = good.clone();
            wrongSpec[14] = target.equals("B") ? "G" : "B";
            for (String[] bad : new String[][] {wrongWorkers, wrongDump, wrongSpec, Arrays.copyOf(good, good.length - 1)}) {
                try {
                    adapter.handleParameters(bad);
                    throw new AssertionError("unexpected arguments accepted");
                } catch (IllegalStateException expected) {
                    // Unsupported argument lists fail before Go is invoked.
                }
            }
        }
        System.setProperty("tla.coverage.target", "other");
        try {
            new TLC().handleParameters(new String[0]);
            throw new AssertionError("unexpected target accepted");
        } catch (IllegalArgumentException expected) {
            // Unsupported targets fail before Go is invoked.
        }
    }
}
