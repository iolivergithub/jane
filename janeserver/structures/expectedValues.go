package structures

type ExpectedValue struct {
	ItemID      string `json:"itemid" bson:"itemid"`
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`

	ElementID    string `json:"elementid" bson:"elementid"`
	EndpointName string `json:"endpointname" bson:"endpointname"`
	IntentID     string `json:"intentid" bson:"intentid"`

	EVS map[string]interface{} `json:"evs" bson:"evs"`

	RecordHistory RecordHistory `json:"recordhistory,omitempty" bson:"recordhistory,omitempty"  yaml:"recordhistory,omitempty"`
}
