package tooling

import (
	"bufio"
	"os"
	"strings"
)

type ProjectConfig struct {
	PackageName  string
	Version      string
	Authors      []string
	Entrypoint   string
	Dependencies map[string]string
}

func ParseToml(path string) (*ProjectConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &ProjectConfig{
		Dependencies: make(map[string]string),
		Entrypoint:   "main.aeth",
		Version:      "1.0.0",
	}

	scanner := bufio.NewScanner(file)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

			switch currentSection {
			case "package", "":
				switch k {
				case "name":
					cfg.PackageName = v
				case "version":
					cfg.Version = v
				case "entrypoint", "main":
					cfg.Entrypoint = v
				}
			case "dependencies":
				cfg.Dependencies[k] = v
			}
		}
	}

	return cfg, nil
}
