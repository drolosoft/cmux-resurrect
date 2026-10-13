package persist

import (
	"reflect"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

func TestFileStore_ListCarriesGroups(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	grouped := &model.Layout{
		Name:    "grouped",
		SavedAt: time.Now(),
		Groups:  []model.Group{{Name: "🎮 Cmux", Pinned: true}},
		Workspaces: []model.Workspace{
			{Title: "loose", Index: 0},
			{Title: "Group 7", Index: 1, Group: "🎮 Cmux", GroupAnchor: true},
		},
	}
	plain := &model.Layout{
		Name:       "plain",
		SavedAt:    time.Now(),
		Workspaces: []model.Workspace{{Title: "only", Index: 0}},
	}
	for _, layout := range []*model.Layout{grouped, plain} {
		if err := store.Save(layout.Name, layout); err != nil {
			t.Fatalf("Save %s: %v", layout.Name, err)
		}
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if !reflect.DeepEqual(metas[0].Groups, grouped.Groups) {
		t.Errorf("grouped meta groups = %+v, want %+v", metas[0].Groups, grouped.Groups)
	}
	wantMembers := []model.Membership{{Index: 0}, {Index: 1, Group: "🎮 Cmux", Anchor: true}}
	if !reflect.DeepEqual(metas[0].Memberships, wantMembers) {
		t.Errorf("grouped meta memberships = %+v, want %+v", metas[0].Memberships, wantMembers)
	}

	// A layout without groups keeps its metadata, and so its JSON, unchanged.
	if metas[1].Groups != nil || metas[1].Memberships != nil {
		t.Errorf("plain meta should carry no group data, got %+v / %+v", metas[1].Groups, metas[1].Memberships)
	}
}
