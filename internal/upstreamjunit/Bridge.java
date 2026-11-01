import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.Base64;
import org.junit.Test;
import tlc2.TestMPRecorder;
import tlc2.tool.CommonTestCase;
import tlc2.tool.EmptyTest;
import tlc2.tool.suite.Test10;

// Executes the original inherited assertion method on a fresh Test10 instance.
// Setup and teardown are intentionally not claimed by this projection.
public class Bridge {
    public static class UpstreamAssertionFailure extends AssertionError {
        UpstreamAssertionFailure(AssertionError cause) { super(cause); }
    }

    @Test public void observations() throws Exception {
        String target = new String(Files.readAllBytes(Paths.get("bridge-target")), StandardCharsets.UTF_8);
        CommonTestCase test;
        if (target.equals("Test10")) { test = new Test10(); }
        else if (target.equals("EmptyTest")) { test = new EmptyTest(); }
        else { throw new IllegalArgumentException("unsupported target"); }
        java.lang.reflect.Field field = CommonTestCase.class.getDeclaredField("recorder");
        field.setAccessible(true);
        TestMPRecorder recorder = (TestMPRecorder) field.get(test);
        loadRecords(recorder);
        try {
            if (test instanceof Test10) { ((Test10) test).testSpec(); }
            else { ((EmptyTest) test).testSpec(); }
        } catch (AssertionError failure) {
            throw new UpstreamAssertionFailure(failure);
        }
    }

    static void loadRecords(TestMPRecorder recorder) throws Exception {
        for (String line : Files.readAllLines(Paths.get("bridge-records"), StandardCharsets.UTF_8)) {
            String[] parts = line.split("\\t", -1);
            int code = Integer.parseInt(parts[0]);
            if (!(code == 2186 || code == 2190 || code == 2199 || code == 1000 || code == 2221)) {
                throw new IllegalArgumentException("unsupported code " + code);
            }
            String[] values = new String[parts.length - 1];
            for (int i = 1; i < parts.length; i++) {
                values[i - 1] = new String(Base64.getDecoder().decode(parts[i]), StandardCharsets.UTF_8);
            }
            recorder.record(code, (Object[]) values);
        }
    }

    static int loadExit() throws Exception {
        return Integer.parseInt(new String(Files.readAllBytes(Paths.get("bridge-exit")), StandardCharsets.UTF_8));
    }
}
