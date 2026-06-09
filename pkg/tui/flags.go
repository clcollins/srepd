package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/clcollins/srepd/pkg/ocm"
)

// FlagConditionType identifies the kind of flag condition.
type FlagConditionType int

const (
	FlagClusterID FlagConditionType = iota
	FlagOrgName
)

const defaultFlagMarker = "🚩 "

// FlagCondition is a user-defined rule that marks matching incidents.
type FlagCondition struct {
	ID        int               `json:"id"`
	Type      FlagConditionType `json:"type"`
	Pattern   string            `json:"pattern"`
	Label     string            `json:"label"`
	CreatedAt time.Time         `json:"created_at"`
}

// matchGlob matches a value against a simple glob pattern.
// Supported forms: ^STRING (prefix), STRING$ (suffix), STRING* (prefix),
// ^STRING* (prefix), *STRING$ (suffix), plain STRING (contains).
// All matching is case-insensitive.
func matchGlob(pattern, value string) bool {
	if pattern == "" {
		return true
	}

	lowerValue := strings.ToLower(value)

	hasPrefix := strings.HasPrefix(pattern, "^")
	hasSuffix := strings.HasSuffix(pattern, "$")
	hasLeadingStar := strings.HasPrefix(pattern, "*")
	hasTrailingStar := strings.HasSuffix(pattern, "*")

	core := pattern
	if hasPrefix {
		core = core[1:]
	}
	if hasLeadingStar && !hasPrefix {
		core = core[1:]
	}
	if hasSuffix {
		core = core[:len(core)-1]
	}
	if hasTrailingStar && !hasSuffix {
		core = core[:len(core)-1]
	}

	lowerCore := strings.ToLower(core)

	if hasPrefix || hasTrailingStar {
		return strings.HasPrefix(lowerValue, lowerCore)
	}
	if hasSuffix || hasLeadingStar {
		return strings.HasSuffix(lowerValue, lowerCore)
	}

	return strings.Contains(lowerValue, lowerCore)
}

// evaluateFlags checks all incidents against all flag conditions and returns
// a map of incident ID to the IDs of matching conditions.
func evaluateFlags(
	incidents []string,
	conditions []FlagCondition,
	incidentClusterMap map[string][]string,
	clusterCache map[string]*ocm.ClusterInfo,
) map[string][]int {
	if len(conditions) == 0 {
		return nil
	}

	result := make(map[string][]int)

	for _, incidentID := range incidents {
		clusterIDs := incidentClusterMap[incidentID]
		for _, cond := range conditions {
			if matchCondition(cond, clusterIDs, clusterCache) {
				result[incidentID] = append(result[incidentID], cond.ID)
			}
		}
	}

	return result
}

func matchCondition(cond FlagCondition, clusterIDs []string, clusterCache map[string]*ocm.ClusterInfo) bool {
	switch cond.Type {
	case FlagClusterID:
		return matchClusterID(cond.Pattern, clusterIDs, clusterCache)
	case FlagOrgName:
		return matchOrgName(cond.Pattern, clusterIDs, clusterCache)
	default:
		return false
	}
}

func matchClusterID(pattern string, clusterIDs []string, clusterCache map[string]*ocm.ClusterInfo) bool {
	lowerPattern := strings.ToLower(pattern)
	for _, cid := range clusterIDs {
		if strings.ToLower(cid) == lowerPattern {
			return true
		}
		if info, ok := clusterCache[cid]; ok {
			if strings.ToLower(info.ID) == lowerPattern || strings.ToLower(info.ExternalID) == lowerPattern {
				return true
			}
		}
	}
	return false
}

func matchOrgName(pattern string, clusterIDs []string, clusterCache map[string]*ocm.ClusterInfo) bool {
	for _, cid := range clusterIDs {
		if info, ok := clusterCache[cid]; ok && info.Organization != "" {
			if matchGlob(pattern, info.Organization) {
				return true
			}
		}
	}
	return false
}

func (m model) renderFlagConditionsSection() string {
	if m.selectedIncident == nil {
		return ""
	}
	matched, ok := m.flagMatchCache[m.selectedIncident.ID]
	if !ok || len(matched) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n## Flag Conditions\n\n")
	for _, flagID := range matched {
		for _, cond := range m.flagConditions {
			if cond.ID == flagID {
				fmt.Fprintf(&b, "* %s#%d: %s\n", m.flagMarker, cond.ID, cond.Label)
				break
			}
		}
	}
	return b.String()
}

func formatFlagsList(conditions []FlagCondition) string {
	if len(conditions) == 0 {
		return "No active flag conditions.\n\nUse `/flag cluster <id>` or `/flag org <pattern>` to add one."
	}

	var b strings.Builder
	b.WriteString("# Active Flag Conditions\n\n")
	for _, c := range conditions {
		typeName := "unknown"
		switch c.Type {
		case FlagClusterID:
			typeName = "cluster"
		case FlagOrgName:
			typeName = "org"
		}
		fmt.Fprintf(&b, "* **#%d** [%s] %s\n", c.ID, typeName, c.Label)
	}
	b.WriteString("\nUse `/unflag <id>` to remove, `/unflag all` to clear.")
	return b.String()
}

func (m *model) rebuildFlagMatchCache() {
	if len(m.flagConditions) == 0 {
		m.flagMatchCache = nil
		return
	}
	var incidentIDs []string
	for _, inc := range m.incidentList {
		incidentIDs = append(incidentIDs, inc.ID)
	}
	m.flagMatchCache = evaluateFlags(
		incidentIDs,
		m.flagConditions,
		m.incidentClusterMap,
		m.clusterCache,
	)
}
