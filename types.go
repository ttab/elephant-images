package main

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

type ImageConfig struct {
	Images map[string]RepoConfig `yaml:"images"`
}

type RepoConfig struct {
	Source string   `yaml:"source"`
	Tags   []string `yaml:"tags"`
}

func LoadImageConfig(path string) (ImageConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ImageConfig{}, fmt.Errorf("read config file: %w", err)
	}

	var conf ImageConfig

	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		return ImageConfig{}, fmt.Errorf("parse config file: %w", err)
	}

	return conf, nil
}
