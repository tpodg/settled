package task

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

// DecodeConfig decodes raw config into a typed config struct.
func DecodeConfig[T any](raw any) (T, error) {
	var cfg T
	if raw == nil {
		return cfg, nil
	}

	data, err := yaml.Marshal(raw)
	if err != nil {
		return cfg, fmt.Errorf("encode config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

// BuilderFor creates a Builder that decodes config into a typed config before building tasks.
func BuilderFor[T any](key string, build func(T) ([]Task, error)) Builder {
	return Builder{
		Key: key,
		Handler: func(raw any) ([]Task, error) {
			cfg, err := DecodeConfig[T](raw)
			if err != nil {
				return nil, err
			}
			return build(cfg)
		},
	}
}
