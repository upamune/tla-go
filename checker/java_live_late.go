package checker

// TLC builds a tableau node immediately when it first reaches a model state
// whose successors were already explored. That path inserts the stuttering
// transition first and then enumerates actions in model order.
type javaLiveConstruction struct {
	product *search
	order   [][]int
	created []bool
	late    []bool
	stack   []int
	count   int
	current int
}

func (f *fairSearch) orderLateJavaLeadsToNodes(product *search, order [][]int, count int) {
	b := javaLiveConstruction{product: product, order: order, created: make([]bool, len(product.nodes)), late: make([]bool, len(product.nodes)), count: count}
	for _, node := range product.initial {
		b.created[node] = true
	}
	for node := range count {
		b.current = node
		b.process(node)
		b.process(node + count)
	}
	for node := count; node < len(product.nodes); node++ {
		if b.late[node] {
			order[node] = f.javaLateSuccessorOrder(product, node, count)
		}
	}
}

func (b *javaLiveConstruction) process(node int) {
	if !b.created[node] || b.late[node] {
		return
	}
	b.discover(node)
	for len(b.stack) > 0 {
		last := len(b.stack) - 1
		next := b.stack[last]
		b.stack = b.stack[:last]
		b.discover(next)
	}
}

func (b *javaLiveConstruction) discover(node int) {
	for _, choice := range b.order[node] {
		next := b.product.edges[node][choice].to
		if b.created[next] {
			continue
		}
		b.created[next] = true
		if next%b.count <= b.current {
			b.late[next] = true
			b.stack = append(b.stack, next)
		}
	}
}

func (f *fairSearch) javaLateSuccessorOrder(product *search, node, count int) []int {
	edges := product.edges[node]
	indices := make(map[int]int, len(edges))
	for index, edge := range edges {
		indices[edge.to] = index
	}
	order := make([]int, 0, len(edges))
	for _, edge := range f.search.edges[node-count] {
		target := edge.to + count
		if index, ok := indices[target]; ok {
			order = append(order, index)
			delete(indices, target)
		}
	}
	return order
}
