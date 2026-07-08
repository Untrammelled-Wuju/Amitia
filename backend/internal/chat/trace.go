package chat

import (
	"strings"

	promptir "github.com/u-ai/backend/internal/prompt"
	applog "github.com/u-ai/backend/log"
)

const chatTraceStateVersion = "chat-runtime-trace-v1"

func newProcessTrace(requestID, convID, charID, channel string) applog.TraceFields {
	requestID = strings.TrimSpace(requestID)
	return applog.TraceFields{
		RequestID:     requestID,
		CorrelationID: "corr-" + requestID,
		CausationID:   "cause-" + requestID,
		User:          "default",
		Character:     strings.TrimSpace(charID),
		Conversation:  strings.TrimSpace(convID),
		Channel:       strings.TrimSpace(channel),
		StateVersion:  chatTraceStateVersion,
		Path:          "chat.process_message",
	}
}

func updateProcessTraceScope(trace applog.TraceFields, convID, charID, channel string) applog.TraceFields {
	trace.Conversation = strings.TrimSpace(convID)
	trace.Character = strings.TrimSpace(charID)
	trace.Channel = strings.TrimSpace(channel)
	return trace
}

func logPromptTrace(trace applog.TraceFields, pt *promptir.PromptTrace, source string) {
	if pt == nil {
		return
	}

	fields := applog.Fields{
		"prompt_hash":           pt.PromptHash,
		"section_count":         len(pt.Sections),
		"source":                source,
		"persona_section_used":  pt.QualityFlags.PersonaSectionUsed,
		"emotion_section_used":  pt.QualityFlags.EmotionSectionUsed,
		"memory_section_used":   pt.QualityFlags.MemorySectionUsed,
		"intimacy_boundary_used": pt.QualityFlags.IntimacyBoundaryUsed,
		"think_removed":         pt.QualityFlags.ThinkRemoved,
		"markdown_removed":      pt.QualityFlags.MarkdownRemoved,
	}

	var sectionNames []string
	for _, s := range pt.Sections {
		if s.Enabled {
			sectionNames = append(sectionNames, s.SectionName)
		}
	}
	fields["section_names"] = sectionNames

	sectionDetails := make([]map[string]interface{}, 0, len(pt.Sections))
	for _, s := range pt.Sections {
		if !s.Enabled {
			continue
		}
		sectionDetails = append(sectionDetails, map[string]interface{}{
			"section_name":    s.SectionName,
			"source_project":  s.SourceProject,
			"source_file":     s.SourceFile,
			"source_constant": s.SourceConstant,
			"rendered_length": s.RenderedLength,
		})
	}
	fields["section_details"] = sectionDetails

	applog.TraceInfo(trace.WithStage("prompt_trace"), fields, "prompt trace recorded")
}
