package dag

import (
	"fmt"
	"log"
)

type vertex struct {
	name string
}

func (v vertex) equal(w vertex) bool {
	return v.name == w.name
}

func (v vertex) String() string {
	return v.name
}

func newVertex(name string) vertex {
	return vertex{
		name: name,
	}
}

type graph struct {
	// adjacency list
	edges map[vertex][]vertex

	// vertex dup check
	vertexMap map[vertex]struct{}

	// indegree
	indegree map[vertex]int

	// outdegree
	outdegree map[vertex]int

	vcnt int
	ecnt int
}

func newGraph() *graph {
	return &graph{
		edges:     make(map[vertex][]vertex),
		vertexMap: make(map[vertex]struct{}),
		indegree:  make(map[vertex]int),
		outdegree: make(map[vertex]int),
	}
}

// Tarjan 算法涉及到深度优先搜索 DFS 的两个过程：搜索过程和回溯过程。

// DFS 遍历有向图中所有的顶点，并将顶点压入栈
// DFS 搜索过程中，对于从顶点 v 访问顶点 w ,
//   1. 如果顶点 w 没有被访问过，对 w 进行 DFS 遍历
//   2. 如果 w 被访问过且 w 在栈中，更新 low[v] 的值：low[v]=min{low[v], dfn[w]}
// DFS 回溯过程中，对于顶点 v 从顶点 w 回溯，更新 low[v] 的值：low[v]=min{low[v], low[w]}
// 不管是搜索过程还是回溯过程，如果顶点 v 满足 dfn[v] == low[v]，则栈中顶点 v 之上的顶点是一个强连通分量！栈中元素逐个出栈，直到顶点 v 出栈
type scc struct {
	g *graph

	// 标记当前节点是否访问过
	visited map[vertex]struct{}

	// 记录同一个强连通分量中的所有节点
	// 当 DFS 第一次访问顶点时，该顶点入栈；当顶点 v 的强连通分量条件满足时，栈中顶点逐个出栈，直到顶点 v 也出栈
	stack []vertex

	// 设以v为根的子树为subtree(v), low[v]定义为以下结点的dfn的最小值：subtree(v)中的结点；从subtree(v)通过一条不在搜索树上的边能到达的结点
	low map[vertex]int

	// 当前dfs的次数
	time int

	// 深度优先搜索遍历时结点v被搜索的次序
	dfn map[vertex]int

	// 记录当前节点是否在栈中
	instack map[vertex]struct{}

	// 强连通分量的个数
	count int
}

func newScc(g *graph) *scc {
	return &scc{
		g: g,

		visited: make(map[vertex]struct{}),
		low:     make(map[vertex]int),
		stack:   []vertex{},

		dfn:     make(map[vertex]int),
		instack: make(map[vertex]struct{}),
	}
}

func (s *scc) strongComponents() [][]vertex {
	components := [][]vertex{}
	for v := range s.g.vertexMap {
		if _, ok := s.visited[v]; !ok {
			components = s.tarjonscc(components, v)
		}
	}

	return components
}

func (s *scc) tarjonscc(components [][]vertex, v vertex) [][]vertex {
	s.stack = append(s.stack, v)
	s.low[v] = s.time
	s.dfn[v] = s.time
	s.visited[v] = struct{}{}
	s.instack[v] = struct{}{}
	s.time++

	// 按照深度优先搜索算法搜索的次序对图中所有的结点进行搜索。
	// 在搜索过程中，对于结点v和与其相邻的结点w（w 不是 v 的父节点）考虑 3 种情况
	for _, w := range s.g.edges[v] {
		// 1: w未被访问：继续对w进行深度搜索, 在回溯的过程中，用low[v]更新low[w]
		//    因为w是v的子节点，所以在回溯时，v能回溯到的已经在栈中节点，w也肯定能回溯到
		if _, ok := s.visited[w]; !ok {
			components = s.tarjonscc(components, w)
			if s.low[v] > s.low[w] {
				s.low[v] = s.low[w]
			}
		} else if _, ok := s.instack[w]; ok {
			// 2: w被访问过，且已经在栈中，那么直接根据low[v]的定义，使用dfn[w]更新low[v]
			if s.low[v] > s.dfn[w] {
				s.low[v] = s.dfn[w]
			}
		}

		// 3: w被访问过，且不在栈中，说明v的子树已经搜索完毕，其所在的连通分量已经被处理，无需操作
	}

	// 对于一个连通分量图，我们很容易想到，在该连通图中有且仅有一个dfn[v]=low[v]
	// 该结点一定是在深度遍历的过程中，该连通分量中第一个被访问过的结点，
	// 因为它的 DFN 值和 LOW 值最小，不会被该连通分量中的其他结点所影响
	// 所以，在回溯过程中，若dfn[v] == low[v], 则在栈中从v后的节点构成一个scc
	if s.dfn[v] == s.low[v] {
		var comp []vertex
		for {
			n := len(s.stack) - 1
			w := s.stack[n]
			s.stack = s.stack[:n]
			delete(s.instack, w)
			comp = append(comp, w) // 顶点w所在连通分量的集合
			if v == w {
				components = append(components, comp)
				break
			}
		}
		s.count++
	}

	return components
}

func (s *scc) tarjon(components [][]vertex, v vertex) [][]vertex {
	s.stack = append(s.stack, v)
	s.low[v] = s.time
	s.visited[v] = struct{}{}
	s.time++

	newComponent := true

	for _, w := range s.g.edges[v] {
		if _, ok := s.visited[w]; !ok {
			components = s.tarjon(components, w)
		}

		if s.low[v] > s.low[w] {
			s.low[v] = s.low[w]
			newComponent = false
		}
	}

	if !newComponent {
		return components
	}

	var comp []vertex
	for {
		n := len(s.stack) - 1
		w := s.stack[n]
		s.stack = s.stack[:n]
		s.low[w] = int(^uint(0) >> 1) // maxint
		comp = append(comp, w)        // 顶点w所在连通分量的集合
		if v == w {
			components = append(components, comp)
			break
		}
	}
	s.count++

	return components
}

func (g *graph) delEdge(v, w vertex) {
	if neighbors, ok := g.edges[v]; ok {
		i := -1
		for j := 0; j < len(neighbors); j++ {
			if w.equal(neighbors[j]) {
				i = j
				break
			}
		}
		if i >= 0 {
			if len(neighbors) == 1 {
				delete(g.edges, v)
			} else {
				g.edges[v] = append(neighbors[:i], neighbors[i+1:]...)
			}
		} else {
			return
		}
	} else {
		return
	}

	g.ecnt--

	g.outdegree[v]--
	if g.indegree[v] == 0 && g.outdegree[v] == 0 {
		delete(g.vertexMap, v)
		delete(g.indegree, v)
		delete(g.outdegree, v)
		g.vcnt--
	}

	g.indegree[w]--
	if g.indegree[w] == 0 && g.outdegree[w] == 0 {
		delete(g.vertexMap, w)
		delete(g.indegree, w)
		delete(g.outdegree, w)
		g.vcnt--
	}
}

func (g *graph) addEdge(v, w vertex) {
	if neighbors, ok := g.edges[v]; ok {
		for _, ww := range neighbors {
			if w.equal(ww) {
				return
			}
		}
	}

	g.ecnt++

	if _, ok := g.vertexMap[v]; !ok {
		g.vertexMap[v] = struct{}{}
		g.vcnt++
	}

	if _, ok := g.vertexMap[w]; !ok {
		g.vertexMap[w] = struct{}{}
		g.vcnt++
	}

	if _, ok := g.edges[v]; !ok {
		g.edges[v] = []vertex{}
	}

	g.edges[v] = append(g.edges[v], w)

	g.indegree[w]++
	if _, ok := g.indegree[v]; !ok {
		g.indegree[v] = 0
	}

	g.outdegree[v]++
	if _, ok := g.outdegree[w]; !ok {
		g.outdegree[w] = 0
	}
}

// O(V + E)
func (g *graph) acyclic() ([]vertex, bool) {
	indegree := make(map[vertex]int)
	for v, d := range g.indegree {
		indegree[v] = d
	}
	// for v, neighbors := range g.edges {
	// 	for j := 0; j < len(neighbors); j++ {
	// 		w := neighbors[j]
	// 		indegree[w]++
	// 	}

	// 	if _, ok := indegree[v]; !ok {
	// 		indegree[v] = 0
	// 	}
	// }

	queue := []vertex{}
	for v, degree := range indegree {
		if degree == 0 {
			queue = append(queue, v)
		}
	}

	order := make([]vertex, 0, g.vcnt)
	vertexcnt := 0
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		vertexcnt++

		order = append(order, v)

		for j := 0; j < len(g.edges[v]); j++ {
			w := g.edges[v][j]
			indegree[w]--
			if indegree[w] == 0 {
				queue = append(queue, w)
			}
		}
	}

	if vertexcnt == g.vcnt {
		return order, true
	}

	return order, false
}

func (g *graph) print() {
	for v, neighbors := range g.edges {
		for j := 0; j < len(neighbors); j++ {
			w := neighbors[j]
			fmt.Println(v.name, "->", w.name)
		}
	}
}

func main() {
	v1 := newVertex("1")
	v2 := newVertex("2")
	v3 := newVertex("3")
	v4 := newVertex("4")
	v5 := newVertex("5")

	v6 := newVertex("1")

	m := make(map[vertex]struct{})
	m[v1] = struct{}{}
	_, ok := m[v6]
	log.Println(ok)

	g := newGraph()

	g.addEdge(v1, v2)
	g.addEdge(v3, v1)
	g.addEdge(v2, v4)
	g.addEdge(v2, v5)
	//g.addEdge(v6, v2)
	g.addEdge(v2, v3)

	g.print()

	log.Println(g.acyclic())
	log.Println(g.indegree, g.outdegree)

	s := newScc(g)
	comps := s.strongComponents()
	log.Println("strong component count:", s.count)
	for _, comp := range comps {
		if len(comp) > 1 {
			log.Println(comp)
		}
	}
}
