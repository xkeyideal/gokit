package dag

import (
	"reflect"
	"sort"
	"testing"
)

var (
	v1  = newVertex("1")
	v2  = newVertex("2")
	v3  = newVertex("3")
	v4  = newVertex("4")
	v5  = newVertex("5")
	v6  = newVertex("6")
	v7  = newVertex("7")
	v8  = newVertex("8")
	v9  = newVertex("9")
	v10 = newVertex("10")
	v11 = newVertex("11")
	v12 = newVertex("12")
)

// The graph connection
// u1 <------ u4 -----------> u5 ------> u7
// | ↖︎        ^               |      ↗︎  |
// |    ↖︎     |               |    ↗︎    |
// v       ↖︎  |               v  ↗︎      v
// u2 ----- > u3              u6 <------ u8
//
// u9 ----- > u10
// |           ^
// |           |
// v           |
// u11 -----> u12
func buildGraph() *graph {
	g := newGraph()

	g.addEdge(v1, v2)
	g.addEdge(v2, v3)
	g.addEdge(v3, v1)
	g.addEdge(v3, v4)
	g.addEdge(v4, v1)

	g.addEdge(v4, v5)
	g.addEdge(v5, v6)
	g.addEdge(v5, v7)
	g.addEdge(v6, v7)
	g.addEdge(v7, v8)
	g.addEdge(v8, v6)

	g.addEdge(v9, v10)
	g.addEdge(v9, v11)
	g.addEdge(v11, v12)
	g.addEdge(v12, v10)

	return g
}

func TestAddEdge(t *testing.T) {
	g := buildGraph()

	expectedIndegree := map[vertex]int{
		v1:  2,
		v2:  1,
		v3:  1,
		v4:  1,
		v5:  1,
		v6:  2,
		v7:  2,
		v8:  1,
		v9:  0,
		v10: 2,
		v11: 1,
		v12: 1,
	}

	expectedOutdegree := map[vertex]int{
		v1:  1,
		v2:  1,
		v3:  2,
		v4:  2,
		v5:  2,
		v6:  1,
		v7:  1,
		v8:  1,
		v9:  2,
		v10: 0,
		v11: 1,
		v12: 1,
	}

	if g.ecnt != 15 {
		t.Fatalf("graph edge number expected %v, got %v", 15, g.ecnt)
	}

	if g.vcnt != 12 {
		t.Fatalf("graph vertex number expected %v, got %v", 12, g.vcnt)
	}

	for v, degree := range g.indegree {
		if degree != expectedIndegree[v] {
			t.Fatalf("1 %v node indegree expected %v, got %v", v, expectedIndegree[v], degree)
		}
	}

	for v, degree := range expectedIndegree {
		if degree != g.indegree[v] {
			t.Fatalf("2 %v node indegree expected %v, got %v", v, degree, g.indegree[v])
		}
	}

	for v, degree := range g.outdegree {
		if degree != expectedOutdegree[v] {
			t.Fatalf("1 %v node outdegree expected %v, got %v", v, expectedOutdegree[v], degree)
		}
	}

	for v, degree := range expectedOutdegree {
		if degree != g.outdegree[v] {
			t.Fatalf("2 %v node outdegree expected %v, got %v", v, degree, g.outdegree[v])
		}
	}

	// add dup edge, indegree & outdegree not change
	g.addEdge(v1, v2)
	g.addEdge(v2, v3)

	for v, degree := range g.indegree {
		if degree != expectedIndegree[v] {
			t.Fatalf("1 %v node indegree expected %v, got %v", v, expectedIndegree[v], degree)
		}
	}

	for v, degree := range expectedIndegree {
		if degree != g.indegree[v] {
			t.Fatalf("2 %v node indegree expected %v, got %v", v, degree, g.indegree[v])
		}
	}

	for v, degree := range g.outdegree {
		if degree != expectedOutdegree[v] {
			t.Fatalf("1 %v node outdegree expected %v, got %v", v, expectedOutdegree[v], degree)
		}
	}

	for v, degree := range expectedOutdegree {
		if degree != g.outdegree[v] {
			t.Fatalf("2 %v node outdegree expected %v, got %v", v, degree, g.outdegree[v])
		}
	}

	// del an exist edge
	g.delEdge(v3, v1)

	// repeat del
	g.delEdge(v3, v1)

	expectedIndegree[v1]--
	expectedOutdegree[v3]--
	for v, degree := range g.indegree {
		if degree != expectedIndegree[v] {
			t.Fatalf("1 %v node indegree expected %v, got %v", v, expectedIndegree[v], degree)
		}
	}

	for v, degree := range expectedIndegree {
		if degree != g.indegree[v] {
			t.Fatalf("2 %v node indegree expected %v, got %v", v, degree, g.indegree[v])
		}
	}

	for v, degree := range g.outdegree {
		if degree != expectedOutdegree[v] {
			t.Fatalf("1 %v node outdegree expected %v, got %v", v, expectedOutdegree[v], degree)
		}
	}

	for v, degree := range expectedOutdegree {
		if degree != g.outdegree[v] {
			t.Fatalf("2 %v node outdegree expected %v, got %v", v, degree, g.outdegree[v])
		}
	}

	// del v9 all edges the v9 node should be delete
	g.delEdge(v9, v10)
	g.delEdge(v9, v11)

	if g.ecnt != 12 {
		t.Fatalf("graph edge number expected %v, got %v", 12, g.ecnt)
	}

	if g.vcnt != 11 {
		t.Fatalf("graph vertex number expected %v, got %v", 11, g.vcnt)
	}
}

func TestAcyclic(t *testing.T) {
	g := buildGraph()
	_, cycle := g.acyclic()
	if cycle {
		t.Fatalf("graph expected cycle")
	}

	g.delEdge(v3, v1)
	g.delEdge(v3, v4)

	g.delEdge(v8, v6)

	_, cycle = g.acyclic()
	if !cycle {
		t.Fatalf("graph expected acyclic")
	}

	g.addEdge(v5, v3)
	g.addEdge(v3, v4)
	_, cycle = g.acyclic()
	if cycle {
		t.Fatalf("graph expected cycle")
	}
}

func TestScc(t *testing.T) {
	g := buildGraph()

	expectedScc := [][]vertex{
		{v1, v2, v3, v4},
		{v6, v7, v8},
		{v5},
		{v9},
		{v10},
		{v11},
		{v12},
	}

	s := newScc(g)
	comps := s.strongComponents()
	if len(comps) != 7 {
		t.Fatalf("scc number expected %v, got %v", 7, len(comps))
	}

	for _, comp := range comps {
		sort.Slice(comp, func(i, j int) bool { return comp[i].name < comp[j].name })
		ok := false
		for _, expected := range expectedScc {
			if len(comp) == len(expected) {
				sort.Slice(expected, func(i, j int) bool { return expected[i].name < expected[j].name })
				if reflect.DeepEqual(comp, expected) {
					t.Logf("scc expected %v, got %v", expected, comp)
					ok = true
					break
				}
			}
		}

		if !ok {
			t.Fatalf("scc expected %v, got %v", 7, comp)
		}
	}

	g.addEdge(v2, v9)
	g.addEdge(v10, v2)

	expectedScc = [][]vertex{
		{v1, v2, v3, v4, v9, v10, v11, v12},
		{v6, v7, v8},
		{v5},
	}

	s = newScc(g)
	comps = s.strongComponents()
	if len(comps) != 3 {
		t.Fatalf("scc number expected %v, got %v", 5, len(comps))
	}

	for _, comp := range comps {
		sort.Slice(comp, func(i, j int) bool { return comp[i].name < comp[j].name })
		ok := false
		for _, expected := range expectedScc {
			if len(comp) == len(expected) {
				sort.Slice(expected, func(i, j int) bool { return expected[i].name < expected[j].name })
				if reflect.DeepEqual(comp, expected) {
					t.Logf("scc expected %v, got %v", expected, comp)
					ok = true
					break
				}
			}
		}

		if !ok {
			t.Fatalf("scc expected %v, got %v", 7, comp)
		}
	}
}
