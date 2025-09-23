package structures

type RecordHistory struct {
	Created     Timestamp `json:"created,omitempty" bson:"created,omitempty"`
	LastUpdated Timestamp `json:"lastupdated,omitempty" bson:"lastupdated,omitempty"`
	Archived    Timestamp `json:"archived,omitempty" bson:"archived,omitempty"`
	Reason      string    `json:"reason" bson:"reason"`
}
