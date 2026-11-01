import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;
import java.util.Arrays;
import java.util.Base64;
import tlc2.output.MP;
import tlc2.output.MPRecorder;

// Records the pinned Java TLC recorder arguments, independently of tool text.
public class CaptureRecorder {
    public static void main(String[] args) throws Exception {
        final String path = args[0];
        MP.setRecorder(new MPRecorder() {
            public synchronized void record(int code, Object... values) {
                if (!(code == 1000 || code == 2190 || code == 2194 || code == 2199 || code == 2221 || code == 2269 || (code >= 2772 && code <= 2775))) return;
                StringBuilder line = new StringBuilder();
                line.append(code).append('\t');
                line.append(values.getClass() == String[].class ? "strings" : "unsupported-array:" + values.getClass().getName());
                for (Object value : values) {
                    line.append('\t');
                    if (value instanceof String) line.append("s:").append(Base64.getEncoder().encodeToString(((String) value).getBytes(StandardCharsets.UTF_8)));
                    else line.append("unsupported-value:").append(value == null ? "null" : value.getClass().getName());
                }
                line.append('\n');
                try {
                    Files.write(Paths.get(path), line.toString().getBytes(StandardCharsets.UTF_8), StandardOpenOption.CREATE, StandardOpenOption.APPEND);
                } catch (Exception e) { throw new IllegalStateException(e); }
            }
        });
        tlc2.TLC.main(Arrays.copyOfRange(args, 1, args.length));
    }
}
