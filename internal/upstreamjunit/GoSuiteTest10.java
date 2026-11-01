import org.junit.Before;
import tlc2.tool.suite.Test10;

// The original inherited test and teardown run with an adapted setup.
public class GoSuiteTest10 extends Test10 {
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
