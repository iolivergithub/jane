package operations

import (
	"errors"
	"testing"

	"a10/structures"
)

// The checks made before the database is touched (#118).
func TestCheckRelationship(t *testing.T) {
	cases := []struct {
		rel  structures.ElementRelationship
		want error
	}{
		{structures.ElementRelationship{RelationshipType: "hosted-on", ItemID: "host"}, nil},
		{structures.ElementRelationship{RelationshipType: "runs-on", ItemID: "host"}, nil},
		{structures.ElementRelationship{RelationshipType: "lives-on", ItemID: "host"}, ErrorRelationshipType},
		{structures.ElementRelationship{RelationshipType: "", ItemID: "host"}, ErrorRelationshipType},
		{structures.ElementRelationship{RelationshipType: "hosted-on", ItemID: "vm"}, ErrorRelationshipSelf},
	}
	for _, c := range cases {
		err := checkRelationship("vm", c.rel)
		if c.want == nil && err != nil || c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%+v: got %v, want %v", c.rel, err, c.want)
		}
	}
}
