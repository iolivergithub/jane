package structures

type Message struct {
	ItemID    string    `json:"itemid" bson:"itemid" yaml:"itemid"`
	SessionID string    `json:"sessionid" bson:"sessionid" yaml:"sessionid"`
	ElementID string    `json:"elementid" bson:"elementid" yaml:"elementid"`
	Contents  string    `json:"contents" bson:"contents" yaml:"contents"`
	Timestamp Timestamp `json:"timestamp" bson:"timestamp"`
}
