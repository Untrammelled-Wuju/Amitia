// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package generationprompt

type GoldenTestCase struct {
	Name     string
	Mode     string
	Document PromptDocument
}

var GoldenTestCases = []GoldenTestCase{
	{
		Name: "sheet-basic",
		Mode: "sheet",
		Document: PromptDocument{
			SchemaVersion:      1,
			CharacterIdentity:  "测试角色",
			ArtStyle:           "动漫风格",
			CameraConstraint:   "正面视角",
			BackgroundStrategy: "透明背景",
			GridLayout: GridLayoutInput{
				Rows:         2,
				Columns:      4,
				CellCount:    8,
				ReadingOrder: "从左到右，从上到下",
			},
			FramePhases: []FramePhaseInput{
				{Index: 1, Description: "起手姿势"},
				{Index: 2, Description: "抬手"},
			},
		},
	},
	{
		Name: "keyframe-basic",
		Mode: "keyframe",
		Document: PromptDocument{
			SchemaVersion:     1,
			CharacterIdentity: "测试角色",
			ArtStyle:          "水彩风格",
			GridLayout: GridLayoutInput{
				Rows:    1,
				Columns: 3,
			},
		},
	},
	{
		Name: "single-basic",
		Mode: "single",
		Document: PromptDocument{
			SchemaVersion:     1,
			CharacterIdentity: "测试角色",
			ArtStyle:          "动漫风格",
		},
	},
	{
		Name: "legacy-basic",
		Mode: "legacy",
		Document: PromptDocument{
			SchemaVersion:     1,
			CharacterIdentity: "测试角色",
			PoseConstraint:    "站立",
		},
	},
}

func RunGoldenTest(tc GoldenTestCase) (PromptSnapshot, error) {
	switch tc.Mode {
	case "sheet":
		return BuildSheetPrompt(tc.Document)
	case "keyframe":
		return BuildKeyframePrompt(tc.Document)
	case "single":
		return BuildSingleFramePrompt(tc.Document)
	case "legacy":
		return BuildLegacyFramePrompt(tc.Document)
	default:
		return BuildSheetPrompt(tc.Document)
	}
}
