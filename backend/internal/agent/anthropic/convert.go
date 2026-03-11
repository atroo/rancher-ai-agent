package anthropic

import (
	"encoding/json"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	sdk "github.com/anthropics/anthropic-sdk-go"
)

// toMessages converts agent messages to Anthropic SDK message params.
func toMessages(msgs []agent.Message) []sdk.MessageParam {
	out := make([]sdk.MessageParam, 0, len(msgs))

	for _, msg := range msgs {
		var blocks []sdk.ContentBlockParamUnion

		for _, b := range msg.Content {
			switch b.Type {
			case "text":
				blocks = append(blocks, sdk.NewTextBlock(b.Text))

			case "tool_use":
				// Tool use blocks in history are preserved as-is
				blocks = append(blocks, sdk.ContentBlockParamUnion{
					OfToolUse: &sdk.ToolUseBlockParam{
						ID:    b.ToolCallID,
						Name:  b.ToolName,
						Input: json.RawMessage(b.Input),
					},
				})

			case "tool_result":
				blocks = append(blocks, sdk.NewToolResultBlock(
					b.ToolCallID,
					b.Content,
					b.IsError,
				))
			}
		}

		out = append(out, sdk.MessageParam{
			Role:    sdk.MessageParamRole(msg.Role),
			Content: blocks,
		})
	}

	return out
}

// toTools converts agent tool definitions to Anthropic SDK tool params.
func toTools(defs []agent.ToolDefinition) []sdk.ToolUnionParam {
	out := make([]sdk.ToolUnionParam, 0, len(defs))

	for _, d := range defs {
		// Parse parameters JSON into the schema structure
		var props interface{}
		var required []string

		var schema struct {
			Properties interface{} `json:"properties"`
			Required   []string    `json:"required"`
		}
		if err := json.Unmarshal(d.Parameters, &schema); err == nil {
			props = schema.Properties
			required = schema.Required
		}

		out = append(out, sdk.ToolUnionParam{
			OfTool: &sdk.ToolParam{
				Name:        d.Name,
				Description: sdk.String(d.Description),
				InputSchema: sdk.ToolInputSchemaParam{
					Properties: props,
					Required:   required,
				},
			},
		})
	}

	return out
}

// fromContentBlocks converts Anthropic SDK content blocks to agent content blocks.
func fromContentBlocks(blocks []sdk.ContentBlockUnion) []agent.ContentBlock {
	out := make([]agent.ContentBlock, 0, len(blocks))

	for _, b := range blocks {
		switch b.Type {
		case "text":
			out = append(out, agent.ContentBlock{
				Type: "text",
				Text: b.Text,
			})

		case "tool_use":
			inputJSON, _ := json.Marshal(b.Input)
			out = append(out, agent.ContentBlock{
				Type:       "tool_use",
				ToolCallID: b.ID,
				ToolName:   b.Name,
				Input:      inputJSON,
			})
		}
	}

	return out
}

// fromStopReason converts the Anthropic stop reason to a string.
func fromStopReason(reason sdk.StopReason) string {
	switch reason {
	case sdk.StopReasonEndTurn:
		return "end_turn"
	case sdk.StopReasonToolUse:
		return "tool_use"
	case sdk.StopReasonMaxTokens:
		return "max_tokens"
	default:
		return string(reason)
	}
}
