package template

import (
	"fmt"
	"os"
	"testing"

	"github.com/fm39hz/gotomux/internal/store"
)

// TestMigrateUserSetup rewrites DB shapes + config from current product rules.
// Run: MIGRATE_USER=1 go test ./internal/template/ -run TestMigrateUserSetup -v
func TestMigrateUserSetup(t *testing.T) {
	if os.Getenv("MIGRATE_USER") != "1" {
		t.Skip("MIGRATE_USER=1 required")
	}
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ids, err := st.ListShapes()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		body, ok := st.GetShape(id)
		if !ok {
			continue
		}
		clean := normalizeShapeBody(id, body)
		if clean == "" {
			// still force ToShape path
			p, err := Parse(body)
			if err != nil {
				fmt.Println("skip parse", id, err)
				continue
			}
			pure := ToShape(p, id)
			pure.Name = id
			clean = Format(pure)
		}
		pure := mustParseShape(id, clean)
		if err := st.UpsertShapeByID(id, ShapeKey(pure), clean); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		// re-learn forks from pure shape as preset-like
		// ObserveForks expects instance-ish; pass pure with empty cwd
		ObserveForks(st, pure)
		fmt.Printf("migrated %s → %s\n", id, ShapeLabel(pure))
	}
	// sticky pointer may still be valid id
	if sid := st.StickyID(); sid != "" {
		fmt.Println("sticky:", sid, "label:", StickyLabel(st))
	}
	reconcileConfigShapes(st)
	fmt.Println("reconcile done")
}
