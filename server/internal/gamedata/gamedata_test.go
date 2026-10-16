package gamedata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoDataDir is shared/data relative to this package.
const repoDataDir = "../../../shared/data"

func TestLoadRepositoryData(t *testing.T) {
	d, err := Load(repoDataDir)
	if err != nil {
		t.Fatal(err)
	}
	sword := d.Equipment["TEST-SWORD-001"]
	if sword.Manufacture["铁锭"] != 10 || sword.Profession != "锻造" || d.RepairMaxLoss != 2 {
		t.Fatalf("unexpected sword spec %+v", sword)
	}
	if d.StackLimits[ClassMaterial] != 100 || d.Items["缝线"].WeightCenti != 10 {
		t.Fatal("unexpected stack or weight data")
	}
	if d.Economy.Pool.Cap != 3000 || d.Economy.OrderRewards["测试铁剑"] != 200 {
		t.Fatalf("unexpected economy %+v", d.Economy)
	}
}

func TestLoadRejectsBrokenData(t *testing.T) {
	cases := map[string]struct{ file, old, new, want string }{
		"schema":        {"items.json", `"schema_version": 1`, `"schema_version": 2`, "schema_version 2"},
		"status":        {"economy.json", `"status": "test"`, `"status": "final"`, "only test data"},
		"unknown item":  {"recipes.json", `"木柄坯": 1`, `"木柄": 1`, `unknown item "木柄"`},
		"duplicate":     {"items.json", `{"id": "矿石"`, `{"id": "铁锭"`, "duplicate item"},
		"reward target": {"economy.json", `"测试铁剑": 200`, `"测试铜剑": 200`, "unknown equipment name"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range []string{"items.json", "recipes.json", "economy.json"} {
				b, err := os.ReadFile(filepath.Join(repoDataDir, f))
				if err != nil {
					t.Fatal(err)
				}
				s := string(b)
				if f == tc.file {
					if !strings.Contains(s, tc.old) {
						t.Fatalf("fixture text %q missing from %s", tc.old, f)
					}
					s = strings.Replace(s, tc.old, tc.new, 1)
				}
				if err := os.WriteFile(filepath.Join(dir, f), []byte(s), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Load(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want containing %q", err, tc.want)
			}
		})
	}
}
