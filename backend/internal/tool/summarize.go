package tool

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SummarizePrometheusResult produces a compact summary of a Prometheus API response.
func SummarizePrometheusResult(raw string, filePath string) string {
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
				Values [][]any           `json:"values"`
			} `json:"result"`
		} `json:"data"`
		Error     string `json:"error"`
		ErrorType string `json:"errorType"`
	}

	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Sprintf("Stored raw response in %s (%d bytes). Could not parse as Prometheus response.", filePath, len(raw))
	}

	if resp.Status != "success" {
		return fmt.Sprintf("Prometheus query failed: %s (%s). Full response in %s", resp.Error, resp.ErrorType, filePath)
	}

	resultCount := len(resp.Data.Result)
	if resultCount == 0 {
		return fmt.Sprintf("Query returned 0 results (type: %s). Full response in %s", resp.Data.ResultType, filePath)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Query returned %d %s result(s). Full data stored in %s\n\n", resultCount, resp.Data.ResultType, filePath))

	limit := min(10, resultCount)
	sb.WriteString("Top results:\n")
	for i := range limit {
		r := resp.Data.Result[i]
		metricLabel := formatMetricLabels(r.Metric)

		switch resp.Data.ResultType {
		case "vector":
			if len(r.Value) >= 2 {
				sb.WriteString(fmt.Sprintf("  %s → %v\n", metricLabel, r.Value[1]))
			}
		case "matrix":
			if len(r.Values) > 0 {
				last := r.Values[len(r.Values)-1]
				val := ""
				if len(last) >= 2 {
					val = fmt.Sprintf("%v", last[1])
				}
				sb.WriteString(fmt.Sprintf("  %s → %d data points, latest: %s\n", metricLabel, len(r.Values), val))
			}
		default:
			sb.WriteString(fmt.Sprintf("  %s\n", metricLabel))
		}
	}

	if resultCount > limit {
		sb.WriteString(fmt.Sprintf("  ... and %d more. Use query_json to explore.\n", resultCount-limit))
	}

	return sb.String()
}

// SummarizeTempoSearchResult produces a compact summary of a Tempo search response.
func SummarizeTempoSearchResult(raw string, filePath string) string {
	var resp struct {
		Traces []struct {
			TraceID         string `json:"traceID"`
			RootServiceName string `json:"rootServiceName"`
			RootTraceName   string `json:"rootTraceName"`
			DurationMs      int    `json:"durationMs"`
		} `json:"traces"`
	}

	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Sprintf("Stored raw response in %s (%d bytes). Could not parse as Tempo response.", filePath, len(raw))
	}

	traceCount := len(resp.Traces)
	if traceCount == 0 {
		return fmt.Sprintf("Search returned 0 traces. Full response in %s", filePath)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d trace(s). Full data stored in %s\n\n", traceCount, filePath))

	limit := min(10, traceCount)
	sb.WriteString("Traces:\n")
	for i := range limit {
		t := resp.Traces[i]
		traceIDShort := t.TraceID
		if len(traceIDShort) > 12 {
			traceIDShort = traceIDShort[:12]
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s / %s — %dms\n",
			traceIDShort, t.RootServiceName, t.RootTraceName, t.DurationMs))
	}

	if traceCount > limit {
		sb.WriteString(fmt.Sprintf("  ... and %d more. Use query_json to explore.\n", traceCount-limit))
	}

	return sb.String()
}

// SummarizeTraceResult produces a compact summary of a full trace from Tempo.
func SummarizeTraceResult(raw string, filePath string) string {
	var resp struct {
		Batches []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
			ScopeSpans []struct {
				Spans []struct {
					Name   string `json:"name"`
					SpanId string `json:"spanId"`
				} `json:"spans"`
			} `json:"scopeSpans"`
		} `json:"batches"`
	}

	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Sprintf("Stored full trace in %s (%d bytes). Could not parse structure.", filePath, len(raw))
	}

	var totalSpans int
	var services []string
	serviceSet := make(map[string]bool)

	for _, batch := range resp.Batches {
		for _, attr := range batch.Resource.Attributes {
			if attr.Key == "service.name" && !serviceSet[attr.Value.StringValue] {
				services = append(services, attr.Value.StringValue)
				serviceSet[attr.Value.StringValue] = true
			}
		}
		for _, scope := range batch.ScopeSpans {
			totalSpans += len(scope.Spans)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Trace with %d span(s) across %d service(s). Full data stored in %s\n", totalSpans, len(services), filePath))
	if len(services) > 0 {
		sb.WriteString(fmt.Sprintf("Services: %s\n", strings.Join(services, ", ")))
	}
	sb.WriteString("Use query_json to drill into specific spans.\n")

	return sb.String()
}

// SummarizePodLogs produces a compact summary of pod logs.
func SummarizePodLogs(raw string, filePath string) string {
	lines := strings.Split(raw, "\n")
	lineCount := len(lines)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Fetched %d log lines. Full logs stored in %s\n\n", lineCount, filePath))

	previewStart := max(0, lineCount-5)
	sb.WriteString("Last lines:\n")
	for _, line := range lines[previewStart:] {
		if line != "" {
			truncated := line
			if len(truncated) > 200 {
				truncated = truncated[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("  %s\n", truncated))
		}
	}

	sb.WriteString("\nUse search_in_file to find errors, or read_file with offset/limit to paginate.")

	return sb.String()
}

func formatMetricLabels(m map[string]string) string {
	if name, ok := m["__name__"]; ok {
		var parts []string
		for k, v := range m {
			if k != "__name__" {
				parts = append(parts, fmt.Sprintf("%s=%q", k, v))
			}
		}
		if len(parts) > 0 {
			return fmt.Sprintf("%s{%s}", name, strings.Join(parts, ","))
		}
		return name
	}

	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%q", k, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}
