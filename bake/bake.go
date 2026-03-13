// Package bake parses mybuild declarative config files.
//
// Supported formats:
//   - HCL:  kforge.hcl  (preferred)
//   - JSON: kforge.json
//
// Example kforge.hcl:
//
//	variable "TAG" { default = "latest" }
//
//	target "app" {
//	  context    = "."
//	  dockerfile = "Dockerfile"
//	  platforms  = ["linux/amd64", "linux/arm64"]
//	  tags       = ["myrepo/app:${TAG}"]
//	  cache-from = ["type=registry,ref=myrepo/app:cache"]
//	  cache-to   = ["type=registry,ref=myrepo/app:cache,mode=max"]
//	  push       = true
//	}
//
//	group "default" {
//	  targets = ["app"]
//	}
package bake

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// DefaultFiles is the lookup order for config files.
var DefaultFiles = []string{
	"kforge.hcl",
	"kforge.json",
}

// Target represents a single build target.
type Target struct {
	Name       string   `hcl:"name,label"    json:"name,omitempty"`
	Context    string   `hcl:"context,optional"    json:"context,omitempty"`
	Dockerfile string   `hcl:"dockerfile,optional" json:"dockerfile,omitempty"`
	Platforms  []string `hcl:"platforms,optional"  json:"platforms,omitempty"`
	Tags       []string `hcl:"tags,optional"       json:"tags,omitempty"`
	CacheFrom  []string `hcl:"cache-from,optional" json:"cache-from,omitempty"`
	CacheTo    []string `hcl:"cache-to,optional"   json:"cache-to,omitempty"`
	BuildArgs  []string `hcl:"build-args,optional" json:"build-args,omitempty"`
	Target     string   `hcl:"target,optional"     json:"target,omitempty"`
	Push       bool     `hcl:"push,optional"       json:"push,omitempty"`
	Load       bool     `hcl:"load,optional"       json:"load,omitempty"`
	NoCache    bool     `hcl:"no-cache,optional"   json:"no-cache,omitempty"`
	Secrets    []string `hcl:"secrets,optional"    json:"secrets,omitempty"`
}

// Group is a named collection of targets.
type Group struct {
	Name    string   `hcl:"name,label" json:"name,omitempty"`
	Targets []string `hcl:"targets"    json:"targets"`
}

// Variable defines a configurable variable with an optional default.
type Variable struct {
	Name    string `hcl:"name,label"`
	Default string `hcl:"default,optional"`
}

// File represents the full parsed config.
type File struct {
	Variables []Variable `hcl:"variable,block"`
	Targets   []Target   `hcl:"target,block"`
	Groups    []Group    `hcl:"group,block"`
}

// Load finds and parses the first config file that exists.
// Returns an error if no config file is found.
func Load(overrideFile string) (*File, error) {
	candidates := DefaultFiles
	if overrideFile != "" {
		candidates = []string{overrideFile}
	}

	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return parseFile(f)
		}
	}
	return nil, fmt.Errorf("no config file found (looked for: %s)", strings.Join(candidates, ", "))
}

// parseFile dispatches to the appropriate parser based on file extension.
func parseFile(path string) (*File, error) {
	if strings.HasSuffix(path, ".json") {
		return parseJSON(path)
	}
	return parseHCL(path)
}

// parseHCL parses an HCL config file.
func parseHCL(path string) (*File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	hclFile, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("HCL parse error in %s: %s", path, diags.Error())
	}

	// Build eval context with environment variables
	vars := map[string]cty.Value{}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			vars[parts[0]] = cty.StringVal(parts[1])
		}
	}

	evalVars := map[string]cty.Value{
		"env": cty.ObjectVal(vars),
	}

	body, ok := hclFile.Body.(*hclsyntax.Body)
	if ok {
		envCtx := &hcl.EvalContext{Variables: map[string]cty.Value{"env": cty.ObjectVal(vars)}}
		for _, block := range body.Blocks {
			if block.Type != "variable" || len(block.Labels) == 0 {
				continue
			}

			name := block.Labels[0]
			if attr, ok := block.Body.Attributes["default"]; ok {
				val, diags := attr.Expr.Value(envCtx)
				if diags.HasErrors() {
					return nil, fmt.Errorf("HCL variable default error in %s for %q: %s", path, name, diags.Error())
				}
				if val.Type() == cty.String {
					evalVars[name] = val
				}
			}

			if envVal, exists := os.LookupEnv(name); exists {
				evalVars[name] = cty.StringVal(envVal)
			}
		}
	}

	evalCtx := &hcl.EvalContext{Variables: evalVars}

	var f File
	if diags := gohcl.DecodeBody(hclFile.Body, evalCtx, &f); diags.HasErrors() {
		return nil, fmt.Errorf("HCL decode error in %s: %s", path, diags.Error())
	}
	return &f, nil
}

// jsonFile is a helper struct for JSON parsing where targets/groups are maps.
type jsonFile struct {
	Variable map[string]struct {
		Default string `json:"default"`
	} `json:"variable"`
	Target map[string]Target `json:"target"`
	Group  map[string]struct {
		Targets []string `json:"targets"`
	} `json:"group"`
}

// parseJSON parses a JSON config file.
func parseJSON(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var jf jsonFile
	if err := json.Unmarshal(data, &jf); err != nil {
		return nil, fmt.Errorf("JSON parse error in %s: %w", path, err)
	}

	f := &File{}

	for name, v := range jf.Variable {
		f.Variables = append(f.Variables, Variable{Name: name, Default: v.Default})
	}
	for name, t := range jf.Target {
		t.Name = name
		f.Targets = append(f.Targets, t)
	}
	for name, g := range jf.Group {
		f.Groups = append(f.Groups, Group{Name: name, Targets: g.Targets})
	}
	return f, nil
}

// ResolveTargets returns the list of targets to build.
// If names is empty it uses the "default" group, falling back to all targets.
func (f *File) ResolveTargets(names []string) ([]Target, error) {
	targetMap := map[string]Target{}
	for _, t := range f.Targets {
		targetMap[t.Name] = t
	}
	groupMap := map[string]Group{}
	for _, g := range f.Groups {
		groupMap[g.Name] = g
	}

	if len(names) == 0 {
		if g, ok := groupMap["default"]; ok {
			names = g.Targets
		} else {
			// Build all targets
			return f.Targets, nil
		}
	}

	var result []Target
	for _, name := range names {
		if g, ok := groupMap[name]; ok {
			for _, tname := range g.Targets {
				t, ok := targetMap[tname]
				if !ok {
					return nil, fmt.Errorf("target %q referenced in group %q not found", tname, name)
				}
				result = append(result, t)
			}
		} else if t, ok := targetMap[name]; ok {
			result = append(result, t)
		} else {
			return nil, fmt.Errorf("target or group %q not found", name)
		}
	}
	return result, nil
}

// ApplySet applies a --set override in the form "target.field=value".
func (f *File) ApplySet(overrides []string) error {
	for _, o := range overrides {
		parts := strings.SplitN(o, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --set value %q: expected target.field=value", o)
		}
		keyParts := strings.SplitN(parts[0], ".", 2)
		if len(keyParts) != 2 {
			return fmt.Errorf("invalid --set key %q: expected target.field", parts[0])
		}
		targetName, field, value := keyParts[0], keyParts[1], parts[1]

		for i, t := range f.Targets {
			if t.Name != targetName {
				continue
			}
			switch field {
			case "platforms":
				f.Targets[i].Platforms = strings.Split(value, ",")
			case "tags":
				f.Targets[i].Tags = append(f.Targets[i].Tags, value)
			case "context":
				f.Targets[i].Context = value
			case "dockerfile":
				f.Targets[i].Dockerfile = value
			case "target":
				f.Targets[i].Target = value
			case "push":
				f.Targets[i].Push = value == "true"
			case "no-cache":
				f.Targets[i].NoCache = value == "true"
			case "cache-from":
				f.Targets[i].CacheFrom = append(f.Targets[i].CacheFrom, value)
			case "cache-to":
				f.Targets[i].CacheTo = append(f.Targets[i].CacheTo, value)
			default:
				return fmt.Errorf("unknown field %q in --set override", field)
			}
		}
	}
	return nil
}
