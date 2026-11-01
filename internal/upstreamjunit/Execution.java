import java.nio.charset.StandardCharsets;
import java.util.Base64;
import org.junit.runner.Description;
import org.junit.runner.JUnitCore;
import org.junit.runner.Request;
import org.junit.runner.Result;
import org.junit.runner.notification.Failure;
import org.junit.runner.notification.RunListener;

// One fresh JVM per case: upstream TLC changes process-global state.
public class Execution {
    private static void emit(String kind, String value) {
        System.out.println("EXECUTION\t" + kind + "\t" + Base64.getEncoder().encodeToString(value.getBytes(StandardCharsets.UTF_8)));
    }
    public static void main(String[] args) throws Exception {
        Class<?> cls = Class.forName(args[0]);
        JUnitCore core = new JUnitCore();
        core.addListener(new RunListener() {
            public void testStarted(Description d) { emit("started", d.getDisplayName()); }
            public void testFinished(Description d) { emit("finished", d.getDisplayName()); }
            public void testIgnored(Description d) { emit("ignored", d.getDisplayName()); }
            public void testAssumptionFailure(Failure f) { emit("assumption", f.toString()); }
            public void testFailure(Failure f) { emit("failure", f.getException().getClass().getName() + "\n" + f.getTrace()); }
        });
        Result result = core.run(Request.method(cls, args[1]));
        emit("summary", result.getRunCount() + "," + result.getIgnoreCount() + "," + result.getFailureCount());
        // TLC may leave RMI threads alive after JUnit has completed.
        System.exit(0);
    }
}
