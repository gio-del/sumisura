package claude

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

const importMasterDataSystemPrompt = `You turn the text of a person's existing CV (or a LinkedIn export) into draft Master Data Entries: one Entry per job (or per client engagement, when the CV lists several clients under one employer) and one per personal or side project.

Copy, never invent. Every bullet must be a sentence or fragment copied from the pasted text, verbatim or with only whitespace and bullet markers removed. Do not reword, merge, translate, summarise or add anything, and do not use general knowledge about the employers. Leave a field empty when the text does not state it.

Fields: type is "experience" for a job, "project" for a project. For experience, employer and role are required; kind is "consultancy" when the employer is a consultancy and this Entry is one client's engagement (then client names that client), "in-house" when the employer is itself the job, and empty when unclear. For project, name is required. start and end are a year ("2021") or year-month ("2021-03") exactly as the text gives them; end is empty for a role that is current. tags are technologies or skills the text names for that Entry, spelled as written. Skip education, publications, awards and languages: they are not Entries.

Call the propose_entries tool with your result.`

const importMasterDataToolName = "propose_entries"

func importMasterDataTool() anthropic.ToolUnionParam {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"entries": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":     map[string]any{"type": "string", "enum": []string{"experience", "project"}},
						"employer": str("Experience only: the employer, as written."),
						"kind":     map[string]any{"type": "string", "enum": []string{"", "consultancy", "in-house"}},
						"client":   str("Experience of kind consultancy: the client, as written."),
						"role":     str("Experience only: the job title, as written."),
						"name":     str("Project only: the project name, as written."),
						"location": str("Where the work was, if the text says."),
						"start":    str("YYYY or YYYY-MM."),
						"end":      str("YYYY or YYYY-MM; empty if current."),
						"tags":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"repo":     str("Project only: the repository or link, if the text gives one."),
						"bullets":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Copied from the text."},
					},
					"required": []string{"type", "start", "bullets"},
				},
			},
		},
		Required: []string{"entries"},
	}
	return anthropic.ToolUnionParamOfTool(schema, importMasterDataToolName)
}

// importedEntry is the tool's entry shape: end is a plain string there,
// where masterdata.Entry has a nullable one.
type importedEntry struct {
	masterdata.Entry
	End string `json:"end"`
}

// ImportUserPrompt wraps the pasted CV text.
func ImportUserPrompt(cvText string) string { return "CV text:\n" + cvText }

// ImportedEntries converts the import tool's JSON input into Entries.
func ImportedEntries(input []byte) ([]masterdata.Entry, error) {
	var result struct {
		Entries []importedEntry `json:"entries"`
	}
	if err := json.Unmarshal(input, &result); err != nil {
		return nil, fmt.Errorf("decoding %s tool input: %w", importMasterDataToolName, err)
	}
	entries := make([]masterdata.Entry, 0, len(result.Entries))
	for _, r := range result.Entries {
		e := r.Entry
		if r.End != "" {
			end := r.End
			e.End = &end
		} else {
			e.End = nil
		}
		if e.Tags == nil {
			e.Tags = []string{}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// ProposeEntries asks Claude for draft Entries copied from a pasted CV, via a
// forced call to the propose_entries tool. Nothing it returns is trusted or
// written: the API layer verifies the bullets and the user approves each one.
func (c *Client) ProposeEntries(ctx context.Context, cvText string) ([]masterdata.Entry, error) {
	message, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      c.modelFor(callSiteImportMasterData),
		MaxTokens:  16384,
		System:     []anthropic.TextBlockParam{{Text: importMasterDataSystemPrompt}},
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(ImportUserPrompt(cvText)))},
		Tools:      []anthropic.ToolUnionParam{importMasterDataTool()},
		ToolChoice: anthropic.ToolChoiceParamOfTool(importMasterDataToolName),
	})
	if err != nil {
		return nil, fmt.Errorf("calling Claude API: %w", err)
	}
	c.recordUsage("import_master_data", message.Model, message.Usage, 0)

	for _, block := range message.Content {
		if block.Type == "tool_use" && block.Name == importMasterDataToolName {
			return ImportedEntries(block.Input)
		}
	}
	return nil, fmt.Errorf("Claude response had no %s tool call", importMasterDataToolName)
}
