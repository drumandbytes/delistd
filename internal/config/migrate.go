package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"gopkg.in/yaml.v3"

	"github.com/drumandbytes/delistd/internal/country"
)

// maskPasswords blanks every non-empty password: value in a config document, so
// the 1.x backup never keeps a password in plain text. A document without any
// comes back byte for byte; one it can't parse yields nothing rather than risk
// keeping a password.
func maskPasswords(doc []byte) []byte {
	var root yaml.Node
	if err := yaml.Unmarshal(doc, &root); err != nil {
		return nil
	}
	masked := 0
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if v := n.Content[i+1]; n.Content[i].Value == "password" && v.Kind == yaml.ScalarNode && v.Value != "" {
					v.Value, v.Tag = "", "!!str"
					masked++
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
	if masked == 0 {
		return doc
	}
	out, err := yaml.Marshal(&root)
	if err != nil {
		return nil
	}
	return out
}

// migrateFrom1x upgrades a 1.x config in memory and on disk: the pre-0.11
// top-level profile: block becomes profiles: [{id: default}], and free-text
// profile countries become ISO alpha-2 codes. Before the first rewrite the
// original file is kept as <path>.1x.bak. A country that doesn't resolve is
// left alone and warned about. email.provider needs no step: it is no longer
// read and drops out on the next save.
func migrateFrom1x(path string, orig []byte, cfg *Config, plaintextSecrets bool) {
	changed := plaintextSecrets
	var legacy struct {
		Profile Profile `yaml:"profile"`
	}
	if len(cfg.Profiles) == 0 && yaml.Unmarshal(orig, &legacy) == nil && !reflect.ValueOf(legacy.Profile).IsZero() {
		cfg.Profiles, changed = []NamedProfile{{ID: DefaultProfileID, Profile: legacy.Profile}}, true
	}
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		if p.Country == "" {
			continue
		}
		code, ok := country.Code(p.Country)
		if !ok {
			fmt.Fprintf(os.Stderr, "WARNING: profile %q: country %q isn't a known country; fix it with `delistd profile edit` (country-scoped brokers aren't filtered until then)\n", p.ID, p.Country)
			continue
		}
		if code != p.Country {
			p.Country, changed = code, true
		}
	}
	if !changed {
		return
	}
	bak := path + ".1x.bak"
	if _, err := os.Stat(bak); err != nil {
		masked := maskPasswords(orig)
		if masked == nil {
			fmt.Fprintf(os.Stderr, "WARNING: could not back up config before migrating it from 1.x; leaving it unchanged\n")
			return
		}
		if err := os.WriteFile(bak, masked, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: could not back up config before migrating it from 1.x: %v\n", err)
			return
		}
	}
	if err := Save(path, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not save the migrated config: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Upgraded %s from 1.x (country codes, encrypted passwords). The original is kept as %s with passwords blanked.\n", filepath.Base(path), filepath.Base(bak))
}
