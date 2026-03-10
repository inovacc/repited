package cmd

import (
	"strings"

	"github.com/inovacc/repited/internal/store"
)

// toolMatches returns true if the tool name contains the filter string (case-insensitive).
func toolMatches(tool, filter string) bool {
	return strings.Contains(strings.ToLower(tool), strings.ToLower(filter))
}

// filterToolCounts filters a slice of StoredToolCount, keeping only matching tools.
func filterToolCounts(tools []store.StoredToolCount, filter string) []store.StoredToolCount {
	filtered := make([]store.StoredToolCount, 0, len(tools))

	for _, tc := range tools {
		if toolMatches(tc.Tool, filter) {
			filtered = append(filtered, tc)
		}
	}

	return filtered
}

// filterSequences filters ToolRelation pairs where either From or To matches.
func filterSequences(seqs []store.ToolRelation, filter string) []store.ToolRelation {
	filtered := make([]store.ToolRelation, 0, len(seqs))

	for _, r := range seqs {
		if toolMatches(r.From, filter) || toolMatches(r.To, filter) {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

// filterCooccurrences filters ToolCooccurrence pairs where either ToolA or ToolB matches.
func filterCooccurrences(pairs []store.ToolCooccurrence, filter string) []store.ToolCooccurrence {
	filtered := make([]store.ToolCooccurrence, 0, len(pairs))

	for _, p := range pairs {
		if toolMatches(p.ToolA, filter) || toolMatches(p.ToolB, filter) {
			filtered = append(filtered, p)
		}
	}

	return filtered
}

// filterPositions filters WorkflowStep entries where the tool matches.
func filterPositions(steps []store.WorkflowStep, filter string) []store.WorkflowStep {
	filtered := make([]store.WorkflowStep, 0, len(steps))

	for _, ws := range steps {
		if toolMatches(ws.Tool, filter) {
			filtered = append(filtered, ws)
		}
	}

	return filtered
}

// filterClusterTools filters tools within a ToolCluster, keeping only matching tools.
func filterClusterTools(clusters []store.ToolCluster, filter string) []store.ToolCluster {
	filtered := make([]store.ToolCluster, 0, len(clusters))

	for _, cl := range clusters {
		var matchingTools []store.ClusterTool

		for _, t := range cl.Tools {
			if toolMatches(t.Tool, filter) {
				matchingTools = append(matchingTools, t)
			}
		}

		if len(matchingTools) > 0 {
			filtered = append(filtered, store.ToolCluster{
				Category: cl.Category,
				Tools:    matchingTools,
			})
		}
	}

	return filtered
}
