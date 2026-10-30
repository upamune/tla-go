package checker

import "encoding/json"

// Debug events describe only already-computed data. In particular, they do
// not evaluate expressions or fingerprints, which could alter TLC effects.
func (s *search) debugEvent(event any) {
	if s.model.debug == nil {
		return
	}
	data, err := json.Marshal(event)
	if err == nil {
		s.model.debug(string(data))
	}
}

type debugEdge struct {
	To     int    `json:"to"`
	Action string `json:"action"`
}

func (s *search) debugLivenessGraph() {
	if s.model.debug == nil || len(s.model.livenessOrder) == 0 {
		return
	}
	s.debugEvent(struct {
		Event   string `json:"event"`
		Initial []int  `json:"initial"`
	}{"initial", s.initial})
	for index, node := range s.nodes {
		state := make(map[string]string, len(node.state))
		for name, v := range node.state {
			state[name] = v.String()
		}
		edges := make([]debugEdge, 0, len(s.edges[index]))
		for _, edge := range s.edges[index] {
			edges = append(edges, debugEdge{edge.to, edge.action})
		}
		s.debugEvent(struct {
			Event string            `json:"event"`
			Node  int               `json:"node"`
			State map[string]string `json:"state"`
			Edges []debugEdge       `json:"edges"`
		}{"node", index, state, edges})
	}
}

func (s *search) debugLivenessSuccessors(index int, choices []int) {
	if s.model.debug == nil {
		return
	}
	s.debugEvent(struct {
		Event   string `json:"event"`
		Node    int    `json:"node"`
		Choices []int  `json:"choices"`
	}{"successor-order", index, choices})
}

func (f *fairSearch) debugStates(allowed []bool) {
	if f.search.model.debug == nil {
		return
	}
	for node := range f.search.nodes {
		f.search.debugEvent(struct {
			Event    string `json:"event"`
			Property string `json:"property"`
			Node     int    `json:"node"`
			Bad      bool   `json:"bad"`
			Trigger  bool   `json:"trigger"`
			Allowed  bool   `json:"allowed"`
		}{"fair-state", f.property.name, node, f.bad[node], f.trigger[node], allowed[node]})
	}
}

func (f *fairSearch) debugComponent(nodes []int, valid, pruned bool) {
	if f.search.model.debug == nil {
		return
	}
	f.search.debugEvent(struct {
		Event    string `json:"event"`
		Property string `json:"property"`
		Nodes    []int  `json:"nodes"`
		Valid    bool   `json:"valid"`
		Pruned   bool   `json:"pruned"`
	}{"fair-component", f.property.name, nodes, valid, pruned})
}

type debugWitness struct {
	Node   int    `json:"node"`
	To     *int   `json:"to,omitempty"`
	Action string `json:"action,omitempty"`
}

// Report the selected witnesses without re-evaluating actions or state values.
func (f *fairSearch) debugCycle(start int, witnesses []cycleWitness) {
	if f.search.model.debug == nil {
		return
	}
	selected := make([]debugWitness, 0, len(witnesses))
	for _, witness := range witnesses {
		item := debugWitness{Node: witness.node}
		if witness.edge != nil {
			item.To = &witness.edge.to
			item.Action = witness.edge.action
		}
		selected = append(selected, item)
	}
	f.search.debugEvent(struct {
		Event     string         `json:"event"`
		Property  string         `json:"property"`
		Start     int            `json:"start"`
		Witnesses []debugWitness `json:"witnesses"`
	}{"fair-cycle", f.property.name, start, selected})
}

func (f *fairSearch) debugJavaLeadsToComponent(root int, component []int, count int) {
	if f.search.model.debug == nil {
		return
	}
	nodes := make([]int, len(component))
	for i, node := range component {
		nodes[i] = node % count
	}
	f.search.debugEvent(struct {
		Event    string `json:"event"`
		Property string `json:"property"`
		Root     int    `json:"root"`
		Nodes    []int  `json:"nodes"`
	}{"leads-to-component", f.property.name, root % count, nodes})
}
