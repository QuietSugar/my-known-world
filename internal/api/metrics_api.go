package api

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime"
	"sort"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func (h *Handler) metricsAPI(w http.ResponseWriter, r *http.Request, g *graph.Graph) {
	var buf bytes.Buffer

	// 1. mkw_nodes_total
	nodeTypeCounts := make(map[string]int)
	provisionalCount := 0
	for _, n := range g.Nodes() {
		nodeTypeCounts[string(n.Type)]++
		if n.ReviewStatus == "pending" || n.Type == graph.ProvisionalObject {
			provisionalCount++
		}
	}
	buf.WriteString("# HELP mkw_nodes_total Total number of nodes in graph by type\n")
	buf.WriteString("# TYPE mkw_nodes_total gauge\n")
	nodeTypes := make([]string, 0, len(nodeTypeCounts))
	for typ := range nodeTypeCounts {
		nodeTypes = append(nodeTypes, typ)
	}
	sort.Strings(nodeTypes)
	for _, typ := range nodeTypes {
		fmt.Fprintf(&buf, "mkw_nodes_total{type=%q} %d\n", typ, nodeTypeCounts[typ])
	}
	buf.WriteString("\n")

	// 2. mkw_relations_total
	relationPredicateCounts := make(map[string]int)
	for _, rel := range g.Relations() {
		relationPredicateCounts[string(rel.Predicate)]++
	}
	buf.WriteString("# HELP mkw_relations_total Total number of relations in graph by predicate\n")
	buf.WriteString("# TYPE mkw_relations_total gauge\n")
	predicates := make([]string, 0, len(relationPredicateCounts))
	for p := range relationPredicateCounts {
		predicates = append(predicates, p)
	}
	sort.Strings(predicates)
	for _, p := range predicates {
		fmt.Fprintf(&buf, "mkw_relations_total{predicate=%q} %d\n", p, relationPredicateCounts[p])
	}
	buf.WriteString("\n")

	// 3. mkw_provisional_nodes_total
	buf.WriteString("# HELP mkw_provisional_nodes_total Total number of provisional nodes pending review\n")
	buf.WriteString("# TYPE mkw_provisional_nodes_total gauge\n")
	fmt.Fprintf(&buf, "mkw_provisional_nodes_total %d\n\n", provisionalCount)

	// 4. mkw_graph_revision
	revision, err := g.Fingerprint()
	if err != nil {
		revision = "unknown"
	}
	buf.WriteString("# HELP mkw_graph_revision Current graph revision fingerprint\n")
	buf.WriteString("# TYPE mkw_graph_revision gauge\n")
	fmt.Fprintf(&buf, "mkw_graph_revision{revision=%q} 1\n\n", revision)

	// 5. Standard Go runtime stats
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	buf.WriteString("# HELP go_goroutines Number of goroutines that currently exist\n")
	buf.WriteString("# TYPE go_goroutines gauge\n")
	fmt.Fprintf(&buf, "go_goroutines %d\n\n", runtime.NumGoroutine())

	buf.WriteString("# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use\n")
	buf.WriteString("# TYPE go_memstats_alloc_bytes gauge\n")
	fmt.Fprintf(&buf, "go_memstats_alloc_bytes %d\n\n", ms.Alloc)

	buf.WriteString("# HELP go_memstats_alloc_bytes_total Total number of bytes allocated, even if freed\n")
	buf.WriteString("# TYPE go_memstats_alloc_bytes_total counter\n")
	fmt.Fprintf(&buf, "go_memstats_alloc_bytes_total %d\n\n", ms.TotalAlloc)

	buf.WriteString("# HELP go_memstats_sys_bytes Number of bytes obtained from system\n")
	buf.WriteString("# TYPE go_memstats_sys_bytes gauge\n")
	fmt.Fprintf(&buf, "go_memstats_sys_bytes %d\n\n", ms.Sys)

	buf.WriteString("# HELP go_memstats_mallocs_total Total number of mallocs\n")
	buf.WriteString("# TYPE go_memstats_mallocs_total counter\n")
	fmt.Fprintf(&buf, "go_memstats_mallocs_total %d\n\n", ms.Mallocs)

	buf.WriteString("# HELP go_memstats_frees_total Total number of frees\n")
	buf.WriteString("# TYPE go_memstats_frees_total counter\n")
	fmt.Fprintf(&buf, "go_memstats_frees_total %d\n\n", ms.Frees)

	buf.WriteString("# HELP go_memstats_heap_alloc_bytes Number of heap bytes allocated and still in use\n")
	buf.WriteString("# TYPE go_memstats_heap_alloc_bytes gauge\n")
	fmt.Fprintf(&buf, "go_memstats_heap_alloc_bytes %d\n\n", ms.HeapAlloc)

	buf.WriteString("# HELP go_memstats_heap_sys_bytes Number of heap bytes obtained from system\n")
	buf.WriteString("# TYPE go_memstats_heap_sys_bytes gauge\n")
	fmt.Fprintf(&buf, "go_memstats_heap_sys_bytes %d\n\n", ms.HeapSys)

	buf.WriteString("# HELP go_memstats_heap_idle_bytes Number of heap bytes waiting to be used\n")
	buf.WriteString("# TYPE go_memstats_heap_idle_bytes gauge\n")
	fmt.Fprintf(&buf, "go_memstats_heap_idle_bytes %d\n\n", ms.HeapIdle)

	buf.WriteString("# HELP go_memstats_heap_inuse_bytes Number of heap bytes that are in use\n")
	buf.WriteString("# TYPE go_memstats_heap_inuse_bytes gauge\n")
	fmt.Fprintf(&buf, "go_memstats_heap_inuse_bytes %d\n\n", ms.HeapInuse)

	buf.WriteString("# HELP go_memstats_heap_objects Number of allocated objects\n")
	buf.WriteString("# TYPE go_memstats_heap_objects gauge\n")
	fmt.Fprintf(&buf, "go_memstats_heap_objects %d\n\n", ms.HeapObjects)

	buf.WriteString("# HELP go_memstats_stack_inuse_bytes Number of bytes in use by the stack allocator\n")
	buf.WriteString("# TYPE go_memstats_stack_inuse_bytes gauge\n")
	fmt.Fprintf(&buf, "go_memstats_stack_inuse_bytes %d\n\n", ms.StackInuse)

	buf.WriteString("# HELP go_memstats_stack_sys_bytes Number of bytes obtained from system for stack allocator\n")
	buf.WriteString("# TYPE go_memstats_stack_sys_bytes gauge\n")
	fmt.Fprintf(&buf, "go_memstats_stack_sys_bytes %d\n\n", ms.StackSys)

	buf.WriteString("# HELP go_memstats_num_gc Number of completed GC cycles\n")
	buf.WriteString("# TYPE go_memstats_num_gc counter\n")
	fmt.Fprintf(&buf, "go_memstats_num_gc %d\n", ms.NumGC)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}
