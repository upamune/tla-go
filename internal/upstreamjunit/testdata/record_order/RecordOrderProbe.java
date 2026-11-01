import tlc2.util.FP64;
import tlc2.value.impl.*;
import util.UniqueString;

// Run in a fresh JVM: UniqueString's token table is process-global.
public final class RecordOrderProbe {
  public static void main(String[] args) {
    FP64.Init(0);
    UniqueString first = UniqueString.uniqueStringOf(args[0]);
    UniqueString second = UniqueString.uniqueStringOf(args[1]);
    UniqueString op = UniqueString.uniqueStringOf("op");
    UniqueString adr = UniqueString.uniqueStringOf("adr");
    System.out.println("tokens=" + first + ":" + first.getTok() + "," + second + ":" + second.getTok());
    Value one = IntValue.gen(1), two = IntValue.gen(2);
    Value[] sets = {new SetEnumValue(new Value[]{one, two}, true), new SetEnumValue(new Value[]{one}, true)};
    SetOfRcdsValue set = new SetOfRcdsValue(new UniqueString[]{adr, op}, sets, false);
    ValueEnumeration enumeration = set.elements();
    RecordValue a = (RecordValue) enumeration.nextElement();
    RecordValue b = (RecordValue) enumeration.nextElement();
    System.out.println("uncached=" + a + ";" + b);
    System.out.println("sharedNames=" + (a.names == b.names));
    RecordValue literal = new RecordValue(new UniqueString[]{adr, op}, new Value[]{one, two}, false);
    System.out.println("literalBefore=" + literal);
    literal.fingerPrint(FP64.New());
    System.out.println("literalAfter=" + literal);
    set.fingerPrint(FP64.New()); // convertAndCache
    enumeration = set.elements();
    System.out.println("cached=" + enumeration.nextElement() + ";" + enumeration.nextElement());
    System.out.println("originalAfter=" + a);
  }
}
