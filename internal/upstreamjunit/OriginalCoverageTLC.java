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

// Test-only TLC adapter for the original coverage JUnit lifecycle. Observed
// Go calls are partial and must not be interpreted as a full JUnit pass.
public final class TLC {
    public void setResolver(FilenameToStream resolver) {
        if (resolver == null) { throw new IllegalArgumentException("missing resolver"); }
    }

    public boolean handleParameters(String[] args) {
        String target = System.getProperty("tla.coverage.target");
        if (!"B".equals(target) && !"G".equals(target)) {
            throw new IllegalArgumentException("unsupported coverage target");
        }
        String dump = "${metadir}" + File.separator + "tlc2.tool.coverage." + target + "CoverageTest.dot";
        String[] expected = {"-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "-coverage", "9999", target};
        if (!Arrays.equals(args, expected)) { throw new IllegalStateException("unexpected original setup args: " + Arrays.toString(args)); }
        return true;
    }

    public int process() throws Exception {
        String veto = System.getProperty(ModelChecker.class.getName() + ".vetoCleanup");
        File expected = new File(System.getProperty("basedir"), "test-model/coverage");
        if (!"true".equals(veto) || TLCGlobals.livenessThreshold != Double.MAX_VALUE ||
                !expected.getCanonicalFile().equals(new File(ToolIO.getUserDir()).getCanonicalFile())) {
            throw new IllegalStateException("original setup globals are missing");
        }
        String helper = System.getProperty("tla.coverage.helper");
        String binary = System.getProperty("tla.coverage.binary");
        String model = System.getProperty("tla.coverage.model");
        String target = System.getProperty("tla.coverage.target");
        String mode = System.getProperty("tla.coverage.mode");
        if (helper == null || binary == null || model == null || target == null || mode == null) {
            throw new IllegalStateException("missing bridge configuration");
        }
        ProcessBuilder builder = new ProcessBuilder(helper, "-test.run=^TestOriginalCoverageGoHelper$");
        builder.environment().put("TLA_GO_ORIGINAL_COVERAGE_HELPER", "1");
        builder.environment().put("TLA_GO_ORIGINAL_COVERAGE_BINARY", binary);
        builder.environment().put("TLA_GO_ORIGINAL_COVERAGE_MODEL", model);
        builder.environment().put("TLA_GO_ORIGINAL_COVERAGE_TARGET", target);
        builder.environment().put("TLA_GO_ORIGINAL_COVERAGE_MODE", mode);
        builder.redirectErrorStream(true);
        builder.redirectOutput(new File("coverage-helper.log"));
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
        Files.write(Paths.get("original-coverage-setup-observed"), target.getBytes(StandardCharsets.UTF_8));
        if (mode.equals("exit")) { return EC.TLC_STATE_NOT_COMPLETELY_SPECIFIED_NEXT; }
        return EC.NO_ERROR;
    }
}
