import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintWriter;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;
import java.util.Base64;
import org.junit.Ignore;
import org.junit.Test;
import org.junit.runner.Description;
import org.junit.runner.Request;
import org.junit.runner.Runner;

// Lists runner descriptions without invoking test methods. Runner construction
// may execute user code, such as a Parameterized @Parameters factory.
public class Discovery {
    private static PrintWriter protocol;
    private static String encode(String s) {
        return Base64.getEncoder().encodeToString(s.getBytes(StandardCharsets.UTF_8));
    }
    private static void emit(String type, String owner, String name, String reason) {
        protocol.println("INVENTORY\t" + type + "\t" + encode(owner) + "\t" + encode(name) + "\t" + encode(reason));
    }
    private static void visit(String owner, Description d, Ignore inheritedIgnore, boolean standard, boolean emptySuite) {
        Ignore annotation = d.getAnnotation(Ignore.class);
        Ignore ignored = annotation != null ? annotation : inheritedIgnore;
        if (d.getChildren().isEmpty()) {
            // IgnoredClassRunner describes a class as a childless suite, including
            // when reached through a Suite. It does not enumerate its methods.
            if (emptySuite && d.getMethodName() == null && ignored == null) {
                emit("empty", owner, d.getDisplayName(), "");
            } else if (standard && inheritedIgnore == null && annotation != null && d.getMethodName() == null) {
                emit("ignored-class", owner, d.getDisplayName(), annotation.value());
            } else if ("initializationError".equals(d.getMethodName()) && d.getAnnotation(Test.class) == null) {
                emit("error", owner, "runner initialization error or ambiguous description: " + d.getDisplayName(), "");
            } else if (d.getMethodName() == null || (ignored != null && (!standard || inheritedIgnore != null))) {
                emit("uncertain", owner, d.getDisplayName(), ignored != null ? ignored.value() : "");
            } else {
                emit(ignored != null ? "ignored" : "case", owner, d.getDisplayName(), ignored != null ? ignored.value() : "");
            }
        } else {
            for (Description child : d.getChildren()) visit(owner, child, ignored, standard, false);
        }
    }
    public static void main(String[] args) throws Exception {
        try (PrintWriter output = new PrintWriter(Files.newBufferedWriter(Paths.get(args[0]), StandardCharsets.UTF_8, StandardOpenOption.CREATE_NEW, StandardOpenOption.WRITE))) {
            protocol = output;
            BufferedReader in = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8));
            String name;
            while ((name = in.readLine()) != null) {
                try {
                    Class<?> c = Class.forName(name, false, Discovery.class.getClassLoader());
                    Runner runner = Request.aClass(c).getRunner();
                    if (runner.getClass().getName().equals("org.junit.internal.runners.ErrorReportingRunner")) {
                        emit("error", name, runner.getDescription().toString(), "");
                    } else {
                        String type = runner.getClass().getName();
                        boolean standard = type.equals("org.junit.runners.JUnit4")
                            || type.equals("org.junit.runners.BlockJUnit4ClassRunner")
                            || type.equals("org.junit.internal.builders.IgnoredClassRunner");
                        visit(name, runner.getDescription(), null, standard, type.equals("org.junit.runners.Suite"));
                    }
                } catch (Throwable e) {
                    emit("error", name, e.toString(), "");
                }
            }
            if (output.checkError()) throw new java.io.IOException("failed to write discovery protocol");
        }
    }
}
