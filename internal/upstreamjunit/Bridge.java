import java.nio.charset.StandardCharsets;
import java.nio.ByteBuffer;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.Base64;
import org.junit.Test;
import tlc2.TestMPRecorder;
import tlc2.tool.CommonTestCase;
import tlc2.tool.EmptyTest;
import tlc2.tool.suite.Test10;
import tlc2.tool.coverage.BCoverageTest;

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
        else if (target.equals("BCoverageTest")) { test = new BCoverageTest(); }
        else { throw new IllegalArgumentException("unsupported target"); }
        java.lang.reflect.Field field = CommonTestCase.class.getDeclaredField("recorder");
        field.setAccessible(true);
        TestMPRecorder recorder = (TestMPRecorder) field.get(test);
        loadRecords(recorder);
        try {
            if (test instanceof Test10) { ((Test10) test).testSpec(); }
            else if (test instanceof EmptyTest) { ((EmptyTest) test).testSpec(); }
            else { ((BCoverageTest) test).testSpec(); }
        } catch (AssertionError failure) {
            throw new UpstreamAssertionFailure(failure);
        }
    }

    static void loadRecords(TestMPRecorder recorder) throws Exception {
        for (String line : Files.readAllLines(Paths.get("bridge-records"), StandardCharsets.UTF_8)) {
            String[] parts = line.split("\\t", -1);
            int code = Integer.parseInt(parts[0]);
            if (!Integer.toString(code).equals(parts[0])) { throw new IllegalArgumentException("invalid code encoding"); }
            if (!(code == 2186 || code == 2190 || code == 2194 || code == 2199 || code == 1000 || code == 2221 || (code >= 2772 && code <= 2776))) {
                throw new IllegalArgumentException("unsupported code " + code);
            }
            if (parts.length < 2 || !(parts[1].equals("strings") || parts[1].equals("objects"))) {
                throw new IllegalArgumentException("unsupported array type");
            }
            Object[] values = parts[1].equals("strings") ? new String[parts.length - 2] : new Object[parts.length - 2];
            for (int i = 2; i < parts.length; i++) {
                Object value = decodeValue(parts[i]);
                if (values instanceof String[] && !(value instanceof String)) {
                    throw new IllegalArgumentException("non-string value in String[]");
                }
                values[i - 2] = value;
            }
            recorder.record(code, values);
        }
    }

    static Object decodeValue(String token) throws Exception {
        if (token.length() < 2 || token.charAt(1) != ':') { throw new IllegalArgumentException("invalid typed value"); }
        String value = token.substring(2);
        switch (token.charAt(0)) {
        case 's': return StandardCharsets.UTF_8.newDecoder().decode(ByteBuffer.wrap(Base64.getDecoder().decode(value))).toString();
        case 'i': { Integer n = Integer.valueOf(value); if (n.toString().equals(value)) return n; break; }
        case 'l': { Long n = Long.valueOf(value); if (n.toString().equals(value)) return n; break; }
        case 'b': if (value.equals("true") || value.equals("false")) return Boolean.valueOf(value); break;
        case 'n': if (value.isEmpty()) return null; break;
        }
        throw new IllegalArgumentException("unsupported typed value");
    }

    static int loadExit() throws Exception {
        return Integer.parseInt(new String(Files.readAllBytes(Paths.get("bridge-exit")), StandardCharsets.UTF_8));
    }
}
