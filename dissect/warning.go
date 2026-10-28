package dissect

// ReadWarning records a listener that returned an error while a round was read.
// Read skips that one packet and keeps going, so the rest of the round is still read,
// but whatever that packet carried (a player, a kill, a scoreboard value) may be missing.
// Failures of the same listener on the same pattern with the same error are merged into one warning.
type ReadWarning struct {
	Listener    string `json:"listener"`    // the listener function, e.g. "readPlayer"
	Pattern     string `json:"pattern"`     // the byte pattern it listens for, e.g. "22 07 94 9B DC"
	Error       string `json:"error"`       // e.g. "EOF"
	Count       int    `json:"count"`       // how many packets failed this way
	FirstOffset int    `json:"firstOffset"` // where the first failing pattern match starts in the decompressed round
}
