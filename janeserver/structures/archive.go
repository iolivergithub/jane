package structures

type Archive struct {
	Timestamp Timestamp `json:"timestamp" bson:"timestamp"`
	Reason    string    `json:"reason" bson:"reason"`
}
