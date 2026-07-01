// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

func (s *service) GetCompressionStatus(convID string) map[string]interface{} {
	if s.compressor == nil {
		return map[string]interface{}{}
	}
	return s.compressor.GetCompressionStatus(convID)
}

func (s *service) GetPipelineStatus() interface{} {
	if s.pipeline == nil {
		return nil
	}
	return s.pipeline.LastRun()
}
