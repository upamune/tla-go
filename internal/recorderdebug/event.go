// Package recorderdebug defines a partial, opt-in observation stream.
// It is not a complete Java TLC MPRecorder replacement.
package recorderdebug

// Event is one JSON line. The stream begins with a versioned, incomplete
// header, followed by observed recorder calls in occurrence order.
type Event struct {
	Kind     string   `json:"kind"`
	Version  int      `json:"version,omitempty"`
	Complete bool     `json:"complete"`
	Code     int      `json:"code,omitempty"`
	Array    string   `json:"array,omitempty"`
	Values   []string `json:"values,omitempty"`
	Reason   string   `json:"reason,omitempty"`
}

// SuccessProbabilityOmission signals that TLC's fingerprint distance is unavailable.
const SuccessProbabilityOmission = "observed-fingerprint-probability-unavailable"

// TemporalCheckUncertain marks an earlier 2192 observation as unsafe to use.
const TemporalCheckUncertain = "temporal-check-start-invalidated"
