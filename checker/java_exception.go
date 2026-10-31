package checker

import "github.com/upamune/tla-go/evaluator"

// evalExceptionCodes are the message codes of errors that TLC throws as
// tlc2.tool.EvalException. It raises other coded messages with Assert.fail,
// whose TLCRuntimeException MP.ECGeneralMsg reports as a RuntimeException.
var evalExceptionCodes = map[int]bool{
	2132: true, 2145: true, 2155: true, 2157: true, 2158: true, 2169: true,
	2176: true, 2177: true, 2179: true, 2180: true, 2181: true, 2182: true,
	2183: true, 2184: true, 2266: true, 2283: true,
}

// javaException names the Java exception behind an evaluation error and the
// message code TLC built its message from, or 0 for a plain message.
func javaException(failure *evaluator.RuntimeError) (string, int) {
	code := failure.ToolCode
	if code == 0 && failure.Unexpected != nil {
		code = failure.Unexpected.ToolCode
	}
	if evalExceptionCodes[code] {
		return "tlc2.tool.EvalException", code
	}
	return "java.lang.RuntimeException", code
}

// generalFailure is MP.ECGeneralMsg for the exception behind failure while
// TLC was doing activity. A coded message is nested in tool output.
func generalFailure(activity string, failure *evaluator.RuntimeError) *Failure {
	class, _ := javaException(failure)
	heading := "TLC threw an unexpected exception.\nThis was probably caused by an error in the spec or model.\nThe error occurred when TLC was " + activity + ".\nThe exception was a " + class + "\n: "
	result := &Failure{Message: heading + failure.Message, ToolCode: 1000}
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
