package core

import "testing"

func TestDependencyGraph_ZeroValue(t *testing.T) {
	var g DependencyGraph
	if g.Nodes != nil {
		t.Error("zero-value DependencyGraph.Nodes should be nil (not panic-prone)")
	}
	if g.Edges != nil {
		t.Error("zero-value DependencyGraph.Edges should be nil (not panic-prone)")
	}
}

func TestDependencyGraph_Populated(t *testing.T) {
	g := DependencyGraph{
		Nodes: []string{"A", "B"},
		Edges: map[string][]string{"A": {"B"}},
	}
	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges["A"]) != 1 || g.Edges["A"][0] != "B" {
		t.Errorf("expected edge A→B, got %v", g.Edges["A"])
	}
}
