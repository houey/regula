// Copyright 2021 Fugue, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reporter

import (
	"encoding/json"
	"sort"

	"github.com/fugue/regula/pkg/loader"
	"github.com/open-policy-agent/opa/rego"
)

type Severity int

const (
	Unknown Severity = iota
	Informational
	Low
	Medium
	High
	Critical
	Off
)

var SeverityIds = map[Severity][]string{
	Unknown:       {"unknown"},
	Informational: {"informational"},
	Low:           {"low"},
	Medium:        {"medium"},
	High:          {"high"},
	Critical:      {"critical"},
	Off:           {"off"},
}

type Format int

const (
	JSON Format = iota
	Table
	Junit
	Tap
	None
)

var FormatIds = map[Format][]string{
	JSON:  {"json"},
	Table: {"table"},
	Junit: {"junit"},
	Tap:   {"tap"},
	None:  {"none"},
}

type RegulaOutput struct {
	RuleResults []RuleResult `json:"rule_results"`
	Summary     Summary      `json:"summary"`
}

var regulaSeverities map[string]Severity = map[string]Severity{
	"Unknown":       Unknown,
	"Informational": Informational,
	"Low":           Low,
	"Medium":        Medium,
	"High":          High,
	"Critical":      Critical,
}

func (o RegulaOutput) ExceedsSeverity(severity Severity) bool {
	if o.Summary.RuleResults["FAIL"] < 1 {
		return false
	}
	maxSeverity := Unknown
	for s, count := range o.Summary.Severities {
		if count < 1 {
			continue
		}
		level, ok := regulaSeverities[s]
		if !ok {
			continue
		}
		if level > maxSeverity {
			maxSeverity = level
		}
	}

	return maxSeverity >= severity
}

type ResourceResults struct {
	Filepath     string
	ResourceID   string
	ResourceType string
	Results      []RuleResult
	Pass         bool
}

type FilepathResults struct {
	Filepath string
	Results  map[string]ResourceResults
	Pass     bool
}

func (f FilepathResults) SortedKeys() []string {
	keys := []string{}
	for k := range f.Results {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type ResultsByFilepath map[string]FilepathResults

func (r ResultsByFilepath) SortedKeys() []string {
	keys := []string{}
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (o RegulaOutput) AggregateByFilepath() ResultsByFilepath {
	byFilepath := ResultsByFilepath{}
	for _, r := range o.RuleResults {
		filepathResults, ok := byFilepath[r.Filepath]
		if !ok {
			filepathResults = FilepathResults{
				Filepath: r.Filepath,
				Results:  map[string]ResourceResults{},
				Pass:     !r.IsFail(),
			}
		}
		resourceResults, ok := filepathResults.Results[r.ResourceID]
		if !ok {
			resourceResults = ResourceResults{
				Filepath:     r.Filepath,
				ResourceID:   r.ResourceID,
				ResourceType: r.ResourceType,
				Results:      []RuleResult{},
				Pass:         !r.IsFail(),
			}
		}
		resourceResults.Results = append(resourceResults.Results, r)
		resourceResults.Pass = resourceResults.Pass && !r.IsFail()
		filepathResults.Results[r.ResourceID] = resourceResults
		filepathResults.Pass = filepathResults.Pass && resourceResults.Pass
		byFilepath[r.Filepath] = filepathResults
	}
	return byFilepath
}

type RuleResult struct {
	Controls        []string `json:"controls"`
	Filepath        string   `json:"filepath"`
	Platform        string   `json:"platform"`
	Provider        string   `json:"provider"`
	ResourceID      string   `json:"resource_id"`
	ResourceType    string   `json:"resource_type"`
	RuleDescription string   `json:"rule_description"`
	RuleID          string   `json:"rule_id"`
	RuleMessage     string   `json:"rule_message"`
	RuleName        string   `json:"rule_name"`
	RuleResult      string   `json:"rule_result"`
	RuleSeverity    string   `json:"rule_severity"`
	RuleSummary     string   `json:"rule_summary"`
}

func (r RuleResult) IsWaived() bool {
	return r.RuleResult == "WAIVED"
}

func (r RuleResult) IsPass() bool {
	return r.RuleResult == "PASS"
}

func (r RuleResult) IsFail() bool {
	return r.RuleResult == "FAIL"
}

func (r RuleResult) Message() string {
	if r.RuleMessage != "" {
		return r.RuleMessage
	}
	if r.RuleSummary != "" {
		return r.RuleSummary
	}
	return r.RuleDescription
}

type Summary struct {
	Filepaths   []string       `json:"filepaths"`
	RuleResults map[string]int `json:"rule_results"`
	Severities  map[string]int `json:"severities"`
}

func ParseRegulaOutput(_ loader.LoadedConfigurations, r rego.Result) (*RegulaOutput, error) {
	j, err := json.Marshal(r.Expressions[0].Value)
	if err != nil {
		return nil, err
	}
	output := &RegulaOutput{}
	if err = json.Unmarshal(j, output); err != nil {
		return nil, err
	}
	return output, nil
}

type Reporter func(r *RegulaOutput) (string, error)
