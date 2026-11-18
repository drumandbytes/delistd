package broker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/drumandbytes/delistd/data"
	"github.com/drumandbytes/delistd/internal/datadir"
)

// Parse unmarshals a broker YAML document and drops the invalid URLs from it.
// It is what a freshly downloaded list goes through before replacing the local copy.
func Parse(raw []byte) (*BrokerDatabase, error) {
	var db BrokerDatabase
	if err := yaml.Unmarshal(raw, &db); err != nil {
		return nil, fmt.Errorf("failed to parse broker data: %w", err)
	}
	for i := range db.Brokers {
		dropInvalidURLs(&db.Brokers[i])
	}
	return &db, nil
}

// LoadFromFile parses the broker YAML at path.
func LoadFromFile(path string) (*BrokerDatabase, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read broker file: %w", err)
	}
	return Parse(raw)
}

// UserBrokersPath is where `delistd update-brokers` writes a refreshed list;
// Load prefers it to the embedded one. Empty when there is no home directory.
func UserBrokersPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(datadir.Of(home), "brokers.yaml")
}

// Load resolves the list from, in order: overridePath (the --brokers flag),
// the update-brokers copy, the list embedded in the binary. Everything but an
// explicit --brokers file also gets the user's own entries (local.go) merged on
// top. Nothing is looked up in ./data.
func Load(overridePath string) (*BrokerDatabase, error) {
	if overridePath != "" {
		return LoadFromFile(overridePath)
	}
	return withLocal(loadPublished())
}

// loadPublished is the shared list without the user's own entries.
func loadPublished() (*BrokerDatabase, error) {
	if p := UserBrokersPath(); p != "" {
		if _, err := os.Stat(p); err == nil {
			return LoadFromFile(p)
		}
	}
	return Parse(data.BrokersYAML)
}

// LoadList is Load for the commands that send: --brokers, then
// options.broker_file, then the embedded verified list when listName is
// "verified", else Load. Audit, guides, update-brokers and reply processing call
// Load, never this.
func LoadList(overridePath, configPath, listName string) (*BrokerDatabase, error) {
	switch {
	case overridePath != "":
		return LoadFromFile(overridePath)
	case configPath != "":
		return withLocal(LoadFromFile(configPath))
	case strings.EqualFold(listName, "verified"):
		return withLocal(Parse(data.BrokersVerifiedYAML))
	}
	return Load("")
}
