import org.junit.Before;
import tlc2.tool.EmptyTest;

// JUnit runs EmptyTest.testSpec and the inherited ModelCheckerTestCase.tearDown.
// This deliberately replaces Java TLC setup; it does not establish setup parity.
public class GoEmptyTest extends EmptyTest {
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
