package authz

import "testing"

func TestCatalogHasUniqueKeys(t *testing.T) {
	seen := make(map[string]struct{}, len(Catalog))
	for _, p := range Catalog {
		if p.Key == "" || p.Category == "" || p.Description == "" {
			t.Fatalf("permission %+v has an empty field", p)
		}
		if _, dup := seen[p.Key]; dup {
			t.Fatalf("duplicate permission key %q", p.Key)
		}
		seen[p.Key] = struct{}{}
	}
}

func TestKeys(t *testing.T) {
	keys := Keys()
	for _, expected := range []string{UsersWrite, CustomersRead, InvoicesWrite, ReportsRead} {
		if _, ok := keys[expected]; !ok {
			t.Fatalf("catalog is missing %q", expected)
		}
	}
}
