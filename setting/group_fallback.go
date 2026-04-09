package setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var groupFallbacks = map[string][]string{}

func UpdateGroupFallbacksByJSONString(jsonString string) error {
	if strings.TrimSpace(jsonString) == "" {
		jsonString = "{}"
	}

	parsed := make(map[string][]string)
	if err := common.Unmarshal([]byte(jsonString), &parsed); err != nil {
		return err
	}

	normalized := make(map[string][]string, len(parsed))
	for group, fallbacks := range parsed {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}

		seen := map[string]struct{}{
			group: {},
		}
		items := make([]string, 0, len(fallbacks))
		for _, fallback := range fallbacks {
			fallback = strings.TrimSpace(fallback)
			if fallback == "" {
				continue
			}
			if _, ok := seen[fallback]; ok {
				continue
			}
			seen[fallback] = struct{}{}
			items = append(items, fallback)
		}
		normalized[group] = items
	}

	groupFallbacks = normalized
	return nil
}

func GroupFallbacks2JSONString() string {
	jsonBytes, err := common.Marshal(groupFallbacks)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetGroupFallbacksCopy() map[string][]string {
	copyMap := make(map[string][]string, len(groupFallbacks))
	for group, fallbacks := range groupFallbacks {
		copyMap[group] = append([]string(nil), fallbacks...)
	}
	return copyMap
}

func GetGroupFallbackChain(group string) []string {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil
	}

	result := []string{group}
	visited := map[string]struct{}{
		group: {},
	}
	queue := []string{group}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, fallback := range groupFallbacks[current] {
			fallback = strings.TrimSpace(fallback)
			if fallback == "" {
				continue
			}
			if _, ok := visited[fallback]; ok {
				continue
			}
			visited[fallback] = struct{}{}
			result = append(result, fallback)
			queue = append(queue, fallback)
		}
	}

	return result
}
