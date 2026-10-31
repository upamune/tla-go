package checker

import "github.com/upamune/tla-go/evaluator"

// Java exception classes that TLC's MP.ECGeneralMsg names.
const (
	runtimeExceptionClass         = "java.lang.RuntimeException"
	evalExceptionClass            = "tlc2.tool.EvalException"
	wrongInvocationExceptionClass = "util.WrongInvocationException"
)

// evalExceptionCodes are the message codes of errors that TLC throws as
// tlc2.tool.EvalException. It raises other coded messages with Assert.fail,
// whose TLCRuntimeException MP.ECGeneralMsg reports as a RuntimeException.
var evalExceptionCodes = map[int]bool{
	2132: true, 2145: true, 2155: true, 2157: true, 2158: true, 2169: true,
	2176: true, 2177: true, 2179: true, 2180: true, 2181: true, 2182: true,
	2183: true, 2184: true, 2266: true, 2283: true,
}

// javaException names the Java exception behind an evaluation error and the
// message code TLC built its message from, or 0 for a plain message. A
// -maxSetSize failure is a util.WrongInvocationException.
func javaException(failure *evaluator.RuntimeError) (string, int) {
	code := failure.ToolCode
	if code == 0 && failure.Unexpected != nil {
		code = failure.Unexpected.ToolCode
	}
	switch {
	case failure.WrongInvocation:
		return wrongInvocationExceptionClass, code
	case evalExceptionCodes[code]:
		return evalExceptionClass, code
	}
	return runtimeExceptionClass, code
}

// generalMessage is MP.ECGeneralMsg, TLC's message for an exception of class
// while TLC was doing activity, which can be empty, followed by the
// exception's message.
func generalMessage(activity, class, detail string) string {
	cause := "See the User Output or TLC Console for clues to what happened."
	if activity != "" {
		cause = "The error occurred when TLC was " + activity + "."
	}
	return "TLC threw an unexpected exception.\nThis was probably caused by an error in the spec or model.\n" + cause + "\nThe exception was a " + class + "\n: " + detail
}

// unexpectedExceptionMessage is MP.ECGeneralMsg without an activity, for an
// exception that has no TLC error code.
func unexpectedExceptionMessage(class, detail string) string {
	return generalMessage("", class, detail)
}

// wrongInvocationMessage reports a util.WrongInvocationException, such as a
// -maxSetSize failure, as Java TLC's unexpected exception.
func wrongInvocationMessage(message string) string {
	return unexpectedExceptionMessage(wrongInvocationExceptionClass, message)
}

// generalFailure is MP.ECGeneralMsg for the exception behind failure while
// TLC was doing activity, which can be empty. A coded message is nested in
// tool output.
func generalFailure(activity string, failure *evaluator.RuntimeError) *Failure {
	class, _ := javaException(failure)
	result := &Failure{Message: generalMessage(activity, class, failure.Message), ToolCode: 1000}
	nestJavaMessage(result, failure)
	return result
}

// nestJavaMessage records the coded message of the exception behind failure,
// which TLC's tool output prints in its own envelope.
func nestJavaMessage(result *Failure, failure *evaluator.RuntimeError) {
	if _, code := javaException(failure); code != 0 {
		result.NestedToolCode, result.NestedMessage = code, failure.Message
	}
}
