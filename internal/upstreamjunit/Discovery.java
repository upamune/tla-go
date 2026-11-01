import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import org.junit.Ignore;
import org.junit.Test;
import org.junit.runner.Description;
import org.junit.runner.Request;
import org.junit.runner.Runner;

// Lists runner descriptions without invoking test methods. Runner construction
// may execute user code, such as a Parameterized @Parameters factory.
public class Discovery {
    private static String encode(String s) {
        return Base64.getEncoder().encodeToString(s.getBytes(StandardCharsets.UTF_8));
    }
    private static void emit(String type, String owner, String name) {
        System.out.println("INVENTORY\t" + type + "\t" + encode(owner) + "\t" + encode(name));
    }
    private static void visit(String owner, Description d, boolean ignored) {
        ignored |= d.getAnnotation(Ignore.class) != null;
        if (d.getChildren().isEmpty()) {
            // ErrorReportingRunner creates an unannotated initializationError leaf.
            // A custom runner can create the same description, so without @Test
            // its meaning is ambiguous and must not count as an executable case.
            if ("initializationError".equals(d.getMethodName()) && d.getAnnotation(Test.class) == null) {
                emit("error", owner, "runner initialization error or ambiguous description: " + d.getDisplayName());
            } else {
                emit(ignored ? "ignored" : "case", owner, d.getDisplayName());
            }
        } else {
            for (Description child : d.getChildren()) visit(owner, child, ignored);
        }
    }
    public static void main(String[] args) throws Exception {
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8));
        String name;
        while ((name = in.readLine()) != null) {
            try {
                Class<?> c = Class.forName(name, false, Discovery.class.getClassLoader());
                if (c.getAnnotation(Ignore.class) != null) {
                    emit("ignored-class", name, "class-level @Ignore; JUnit does not enumerate methods");
                    continue;
                }
                Runner runner = Request.aClass(c).getRunner();
                if (runner.getClass().getName().equals("org.junit.internal.runners.ErrorReportingRunner")) {
                    emit("error", name, runner.getDescription().toString());
                } else {
                    visit(name, runner.getDescription(), false);
                }
            } catch (Throwable e) {
                emit("error", name, e.toString());
            }
        }
    }
}
