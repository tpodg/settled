package task

import (
	"embed"
	"fmt"
	"path"
	"sort"

	"github.com/goccy/go-yaml"
)

//go:embed defaults
var defaultsFS embed.FS

type Spec struct {
	Key          string
	DefaultsPath string
	Builder      Builder
}

func SpecFor[T any](key, defaultsPath string, build func(T) ([]Task, error)) Spec {
	return Spec{
		Key:          key,
		DefaultsPath: defaultsPath,
		Builder:      BuilderFor(key, build),
	}
}

// PlanTasks merges defaults with overrides and builds tasks.
func PlanTasks(overrides map[string]any, specs []Spec) ([]Task, []string, error) {
	specIndex := make(map[string]Spec, len(specs))
	builders := make([]Builder, 0, len(specs))
	for _, spec := range specs {
		if _, exists := specIndex[spec.Key]; exists {
			return nil, nil, fmt.Errorf("duplicate task key: %s", spec.Key)
		}
		specIndex[spec.Key] = spec
		builders = append(builders, spec.Builder)
	}

	unknownSet := make(map[string]struct{})
	for key := range overrides {
		if _, ok := specIndex[key]; !ok {
			unknownSet[key] = struct{}{}
		}
	}

	state := make(map[string]any)
	for _, spec := range specs {
		defaults, err := loadDefaults(spec)
		if err != nil {
			return nil, nil, err
		}
		merged := mergeConfig(defaults, overrides[spec.Key])
		state[spec.Key] = merged
	}

	tasks, unknown, err := CreateTasks(state, builders...)
	if err != nil {
		return nil, nil, err
	}

	for _, key := range unknown {
		unknownSet[key] = struct{}{}
	}

	return tasks, setToSortedSlice(unknownSet), nil
}

func loadDefaults(spec Spec) (map[string]any, error) {
	if spec.DefaultsPath == "" {
		return nil, nil
	}

	defaultsPath := path.Join("defaults", spec.DefaultsPath)
	data, err := defaultsFS.ReadFile(defaultsPath)
	if err != nil {
		return nil, fmt.Errorf("read defaults for %s: %w", spec.Key, err)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}

	var defaults map[string]any
	if err := yaml.Unmarshal(data, &defaults); err != nil {
		return nil, fmt.Errorf("parse defaults for %s: %w", spec.Key, err)
	}
	if defaults == nil {
		defaults = map[string]any{}
	}
	return defaults, nil
}

func mergeConfig(defaults map[string]any, override any) any {
	if override == nil {
		if defaults == nil {
			return nil
		}
		return copyMap(defaults)
	}

	overrideMap, ok := override.(map[string]any)
	if !ok {
		return override
	}
	if defaults == nil {
		return copyMap(overrideMap)
	}
	return mergeMaps(defaults, overrideMap)
}

func mergeMaps(base, override map[string]any) map[string]any {
	out := copyMap(base)
	for key, value := range override {
		overrideMap, ok := value.(map[string]any)
		if !ok {
			out[key] = value
			continue
		}

		baseMap, ok := out[key].(map[string]any)
		if !ok {
			out[key] = copyMap(overrideMap)
			continue
		}
		out[key] = mergeMaps(baseMap, overrideMap)
	}
	return out
}

func copyMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func setToSortedSlice(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
