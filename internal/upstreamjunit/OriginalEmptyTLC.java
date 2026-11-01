package tlc2;

import java.io.File;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.Arrays;
import java.util.concurrent.TimeUnit;
import tlc2.output.EC;
import tlc2.output.MP;
import tlc2.tool.ModelChecker;
import util.FilenameToStream;
import util.ToolIO;

// Test-only classpath adapter. The unmodified EmptyTest and its actual JUnit
// setup/teardown are loaded separately. It starts a Go test helper which runs
// tla-go and projects its recorder records; it cannot establish complete
// recorder or TLC process equivalence.
public final class TLC {
    public void setResolver(FilenameToStream resolver) {
        if (resolver == null) { throw new IllegalArgumentException("missing resolver"); }
    }

    public boolean handleParameters(String[] args) throws Exception {
        String dump = "${metadir}" + File.separator + "tlc2.tool.EmptyTest.dot";
        String[] expected = {"-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "-coverage", "1", "Empty"};
        if (!Arrays.equals(args, expected)) { throw new IllegalStateException("unexpected original setup args: " + Arrays.toString(args)); }
        return true;
    }

    public int process() throws Exception {
        String veto = System.getProperty(ModelChecker.class.getName() + ".vetoCleanup");
        File expected = new File(System.getProperty("basedir"), "test-model");
        if (!"true".equals(veto) || TLCGlobals.livenessThreshold != Double.MAX_VALUE ||
                !expected.getCanonicalFile().equals(new File(ToolIO.getUserDir()).getCanonicalFile())) {
            throw new IllegalStateException("original setup globals are missing");
        }
        String helper = System.getProperty("tla.bridge.helper");
        String binary = System.getProperty("tla.bridge.binary");
        String model = System.getProperty("tla.bridge.model");
        String mode = System.getProperty("tla.bridge.mode");
        if (helper == null || binary == null || model == null || mode == null) {
            throw new IllegalStateException("missing bridge configuration");
        }
        ProcessBuilder builder = new ProcessBuilder(helper, "-test.run=^TestOriginalEmptyGoHelper$");
        builder.environment().put("TLA_GO_ORIGINAL_EMPTY_HELPER", "1");
        builder.environment().put("TLA_GO_ORIGINAL_EMPTY_BINARY", binary);
        builder.environment().put("TLA_GO_ORIGINAL_EMPTY_MODEL", model);
        builder.environment().put("TLA_GO_ORIGINAL_EMPTY_MODE", mode);
        builder.redirectErrorStream(true);
        builder.redirectOutput(new File("bridge-helper.log"));
        Process child = builder.start();
        if (!child.waitFor(40, TimeUnit.SECONDS)) {
            child.destroyForcibly();
            throw new IllegalStateException("Go helper timed out");
        }
        if (child.exitValue() != 0) { throw new IllegalStateException("Go helper failed"); }
        Field field = MP.class.getDeclaredField("recorder");
        field.setAccessible(true);
        Object recorder = field.get(null);
        if (!(recorder instanceof TestMPRecorder)) { throw new IllegalStateException("original recorder not installed"); }
        Method load = Class.forName("Bridge").getDeclaredMethod("loadRecords", TestMPRecorder.class);
        load.setAccessible(true);
        load.invoke(null, recorder);
        Files.write(Paths.get("original-setup-observed"), "original EmptyTest setup invoked adapter".getBytes(StandardCharsets.UTF_8));
        if (mode.equals("success") || mode.equals("wrong-stats") || mode.equals("missing-finished") || mode.equals("general") || mode.equals("unknown-tool-code")) { return EC.NO_ERROR; }
        if (mode.equals("exit")) { return EC.TLC_STATE_NOT_COMPLETELY_SPECIFIED_NEXT; }
        throw new IllegalArgumentException("unsupported adapter mode");
    }
}
