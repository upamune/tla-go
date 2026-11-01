package tlc2;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;
import java.util.Base64;

// Called by a temporary instrumented copy of the pinned TestMPRecorder.
// No call is filtered: unknown array or element types make the Go test fail.
public final class FullRecorderCapture {
    public static synchronized void capture(int code, Object[] values) {
        StringBuilder line = new StringBuilder().append(code).append('\t');
        line.append(values.getClass() == String[].class ? "strings" : "unsupported-array");
        for (Object value : values) {
            line.append('\t');
            if (value instanceof String) {
                line.append(Base64.getEncoder().encodeToString(((String) value).getBytes(StandardCharsets.UTF_8)));
            } else {
                line.append("!unsupported-value");
            }
        }
        line.append('\n');
        try {
            Files.write(Paths.get(System.getProperty("tla.full.recorder")), line.toString().getBytes(StandardCharsets.UTF_8),
                    StandardOpenOption.CREATE, StandardOpenOption.APPEND);
        } catch (Exception e) { throw new IllegalStateException(e); }
    }
}
