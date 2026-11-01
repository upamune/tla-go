import org.junit.Before;
import tlc2.tool.coverage.BCoverageTest;

// The original inherited test and teardown run with an adapted setup.
public class GoBCoverageTest extends BCoverageTest {
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
