import static org.junit.Assert.*;
import java.util.List;
import org.junit.Test;
import tlc2.TestMPRecorder;

// Checks the Java recorder's actual retained varargs array and value types.
public class RecorderFixture {
    @Test public void typed() throws Exception {
        TestMPRecorder recorder = new TestMPRecorder();
        Bridge.loadRecords(recorder);
        List<Object> records = recorder.getRecords(2199);
        assertEquals(3, records.size());
        assertEquals(String[].class, records.get(0).getClass());
        assertArrayEquals(new String[] {"first\n\t😀", ""}, (String[]) records.get(0));
        assertEquals(Object[].class, records.get(1).getClass());
        assertArrayEquals(new Object[] {"mixed", Integer.valueOf(-7), Long.valueOf(2147483648L), Boolean.TRUE, null}, (Object[]) records.get(1));
        assertEquals(String[].class, records.get(2).getClass());
        assertArrayEquals(new String[] {"first\n\t😀", ""}, (String[]) records.get(2));
        assertTrue(recorder.recordedWithStringValues(2110, "Inv"));
        assertTrue(recorder.recorded(2114));
        assertTrue(recorder.recorded(2121));
        assertEquals(String[].class, recorder.getRecords(2114).get(0).getClass());
        assertEquals(0, ((String[]) recorder.getRecords(2114).get(0)).length);
    }
}
