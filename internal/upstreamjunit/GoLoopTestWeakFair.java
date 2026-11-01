import org.junit.Before;
import tlc2.tool.liveness.LoopTestWeakFair;

// The original assertion and teardown run with Go recorder observations.
// This replaces Java TLC setup and therefore cannot establish a JUnit pass.
public class GoLoopTestWeakFair extends LoopTestWeakFair {
    @Override @Before public void setUp() {
        try {
            beforeSetUp();
            Bridge.loadRecords(recorder);
            actualExitStatus = Bridge.loadExit();
        } catch (Exception e) {
            throw new IllegalStateException("cannot load Go observations", e);
        }
    }
}
