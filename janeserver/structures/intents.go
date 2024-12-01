package structures

type Intent struct {
	ItemID      string                 `json:"itemid" bson:"itemid"`
	Name        string                 `json:"name" bson:"name"`
	Description string                 `json:"description" bson:"description"`
	Function    string                 `json:"function" bson:"function"`
	Parameters  map[string]interface{} `json:"parameters" bson:"parameters"`
}
