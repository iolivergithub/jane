package structures

type Session struct {
	ItemID string        `json:"itemid" bson:"itemid"`
	Timing SessionTiming `json:"timing" bson:"timing"`

	ClaimList  []string `json:"claimlist" bson:"claimlist"`
	ResultList []string `json:"resultlist" bson:"resultlist"`

	Message string `json:"message" bson:"message"`

	// Parent is the session this one was opened under (#130), set only when
	// it is opened; empty for most sessions.
	Parent string `json:"parent,omitempty" bson:"parent,omitempty"`

	// AttestationProfileRun records the profile run this session belongs to
	// (#130); nil for every other session.
	AttestationProfileRun *ProfileRun `json:"attestationprofilerun,omitempty" bson:"attestationprofilerun,omitempty"`

	Footer SessionFooter `json:"footer" bson:"footer"`
}

type SessionSummary struct {
	ItemID string        `json:"itemid" bson:"itemid"`
	Timing SessionTiming `json:"timing" bson:"timing"`
}

type SessionTiming struct {
	Opened Timestamp `json:"opened" bson:"opened"`
	Closed Timestamp `json:"closed" bson:"closed"`
}

type SessionFooter struct {
	Kid       string `json:"kid" bson:"kid"`
	Alg       string `json:"alg" bson:"alg"`
	Hash      []byte `json:"hash" bson:"hash"`
	Signature []byte `json:"signature" bson:"signature"`
}
