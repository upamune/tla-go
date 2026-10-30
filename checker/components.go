package checker

import "slices"

// components returns the strongly connected components of the induced graph.
// Iterative DFS avoids growing the Go stack with the explored state space.
func (s *search) components(allowed []bool) [][]int {
	seen := make([]bool, len(s.nodes))
	order := make([]int, 0, len(s.nodes))
	for i := range allowed {
		if allowed[i] && !seen[i] {
			s.finishOrder(i, allowed, seen, &order)
		}
	}
	reverse := s.reverseEdges(allowed)
	clear(seen)
	var result [][]int
	for _, o := range slices.Backward(order) {
		if !seen[o] {
			result = append(result, collectReverse(o, reverse, seen))
		}
	}
	return result
}

type dfsFrame struct{ node, edge int }

func (s *search) finishOrder(start int, allowed, seen []bool, order *[]int) {
	seen[start] = true
	stack := []dfsFrame{{node: start}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.edge == len(s.edges[top.node]) {
			*order = append(*order, top.node)
			stack = stack[:len(stack)-1]
			continue
		}
		next := s.edges[top.node][top.edge].to
		top.edge++
		if allowed[next] && !seen[next] {
			seen[next] = true
			stack = append(stack, dfsFrame{node: next})
		}
	}
}

func (s *search) reverseEdges(allowed []bool) [][]int {
	reverse := make([][]int, len(s.nodes))
	for node, edges := range s.edges {
		if !allowed[node] {
			continue
		}
		for _, edge := range edges {
			if allowed[edge.to] {
				reverse[edge.to] = append(reverse[edge.to], node)
			}
		}
	}
	return reverse
}

func collectReverse(start int, reverse [][]int, seen []bool) []int {
	seen[start] = true
	stack := []int{start}
	var component []int
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		component = append(component, node)
		for _, next := range reverse[node] {
			if !seen[next] {
				seen[next] = true
				stack = append(stack, next)
			}
		}
	}
	return component
}
