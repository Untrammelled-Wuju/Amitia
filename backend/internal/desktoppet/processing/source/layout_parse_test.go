package source

import (
	"strings"
	"testing"
)

const segmentedLayoutFixture = `{"Segments":[{"SegmentIndex":0,"SheetLayout":{"Rows":3,"Columns":4,"CellWidth":1024,"CellHeight":1024,"MarginX":0,"MarginY":0,"GapX":0,"GapY":0,"SheetWidth":4096,"SheetHeight":3072,"ReadingOrder":"row_major_left_to_right_top_to_bottom","Cells":[{"Row":0,"Column":0,"FrameIndex":0,"IsEmpty":false},{"Row":0,"Column":1,"FrameIndex":1,"IsEmpty":false},{"Row":0,"Column":2,"FrameIndex":2,"IsEmpty":false},{"Row":0,"Column":3,"FrameIndex":3,"IsEmpty":false},{"Row":1,"Column":0,"FrameIndex":4,"IsEmpty":false},{"Row":1,"Column":1,"FrameIndex":5,"IsEmpty":false},{"Row":1,"Column":2,"FrameIndex":6,"IsEmpty":false},{"Row":1,"Column":3,"FrameIndex":7,"IsEmpty":false},{"Row":2,"Column":0,"FrameIndex":8,"IsEmpty":false},{"Row":2,"Column":1,"FrameIndex":9,"IsEmpty":false},{"Row":2,"Column":2,"FrameIndex":10,"IsEmpty":false},{"Row":2,"Column":3,"FrameIndex":11,"IsEmpty":false}],"EmptyCells":[],"TotalCells":12,"UsedCells":12,"FrameCount":12},"FrameStartIndex":0,"FrameEndIndex":11,"FrameCount":12}],"TotalSegments":1,"TotalSheets":1,"TotalCells":12,"TotalUsedCells":12,"FrameCount":12,"CellWidth":1024,"CellHeight":1024,"SheetWidth":4096,"SheetHeight":3072}`

func TestParseSpriteSheetLayoutSegmented(t *testing.T) {
	layout, err := parseSpriteSheetLayout(segmentedLayoutFixture, 4096, 3072)
	if err != nil {
		t.Fatalf("parse segmented layout failed: %v", err)
	}
	if layout.Rows != 3 || layout.Columns != 4 {
		t.Fatalf("unexpected dimensions: rows=%d cols=%d", layout.Rows, layout.Columns)
	}
	if layout.CellWidth != 1024 || layout.CellHeight != 1024 {
		t.Fatalf("unexpected cell size: %dx%d", layout.CellWidth, layout.CellHeight)
	}
	if layout.ExpectedWidth != 4096 || layout.ExpectedHeight != 3072 {
		t.Fatalf("unexpected expected size: %dx%d", layout.ExpectedWidth, layout.ExpectedHeight)
	}
	if len(layout.Cells) != 12 {
		t.Fatalf("unexpected cell count: %d", len(layout.Cells))
	}
	for i, cell := range layout.Cells {
		if cell.Empty {
			t.Fatalf("cell %d should not be empty", i)
		}
		if cell.FrameIndex == nil || *cell.FrameIndex != i {
			t.Fatalf("cell %d has unexpected frame index", i)
		}
	}
	if err := ValidateLayout(layout, 12); err != nil {
		t.Fatalf("validate layout failed: %v", err)
	}
}

func TestParseSpriteSheetLayoutSegmentedMultiSheet(t *testing.T) {
	payload := `{"Segments":[{"SegmentIndex":0,"SheetLayout":{"Rows":2,"Columns":2,"CellWidth":512,"CellHeight":512,"MarginX":0,"MarginY":0,"GapX":0,"GapY":0,"SheetWidth":1024,"SheetHeight":1024,"ReadingOrder":"row_major_left_to_right_top_to_bottom","Cells":[{"Row":0,"Column":0,"FrameIndex":0,"IsEmpty":false},{"Row":0,"Column":1,"FrameIndex":1,"IsEmpty":false},{"Row":1,"Column":0,"FrameIndex":2,"IsEmpty":false},{"Row":1,"Column":1,"FrameIndex":3,"IsEmpty":false}],"FrameCount":4},"FrameStartIndex":0,"FrameEndIndex":3,"FrameCount":4},{"SegmentIndex":1,"SheetLayout":{"Rows":2,"Columns":2,"CellWidth":256,"CellHeight":256,"MarginX":0,"MarginY":0,"GapX":0,"GapY":0,"SheetWidth":512,"SheetHeight":512,"ReadingOrder":"row_major_left_to_right_top_to_bottom","Cells":[{"Row":0,"Column":0,"FrameIndex":0,"IsEmpty":false},{"Row":0,"Column":1,"FrameIndex":1,"IsEmpty":false},{"Row":1,"Column":0,"FrameIndex":2,"IsEmpty":false},{"Row":1,"Column":1,"FrameIndex":3,"IsEmpty":false}],"FrameCount":4},"FrameStartIndex":4,"FrameEndIndex":7,"FrameCount":4}],"TotalSegments":2,"TotalSheets":2}`
	layout, err := parseSpriteSheetLayout(payload, 512, 512)
	if err != nil {
		t.Fatalf("parse multi-segment layout failed: %v", err)
	}
	if layout.CellWidth != 256 || layout.ExpectedWidth != 512 {
		t.Fatalf("segment matching failed: cellWidth=%d expectedWidth=%d", layout.CellWidth, layout.ExpectedWidth)
	}
}

func TestParseSpriteSheetLayoutFlat(t *testing.T) {
	payload := `{"rows":2,"columns":2,"cellWidth":256,"cellHeight":256,"readingOrder":"row_major_left_to_right_top_to_bottom","cells":[{"row":0,"column":0,"frameIndex":0},{"row":0,"column":1,"frameIndex":1},{"row":1,"column":0,"frameIndex":2},{"row":1,"column":1,"frameIndex":3}]}`
	layout, err := parseSpriteSheetLayout(payload, 512, 512)
	if err != nil {
		t.Fatalf("parse flat layout failed: %v", err)
	}
	if layout.Rows != 2 || layout.Columns != 2 {
		t.Fatalf("unexpected dimensions: rows=%d cols=%d", layout.Rows, layout.Columns)
	}
}

func TestParseSpriteSheetLayoutInvalid(t *testing.T) {
	if _, err := parseSpriteSheetLayout("{}", 0, 0); err == nil {
		t.Fatalf("expected error for empty layout payload")
	} else if !strings.Contains(err.Error(), "no segments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildSpriteSheetFramesFromSegmented(t *testing.T) {
	layout, err := parseSpriteSheetLayout(segmentedLayoutFixture, 4096, 3072)
	if err != nil {
		t.Fatalf("parse segmented layout failed: %v", err)
	}
	artifact := &ArtifactInfo{ArtifactID: "a1", RelativePath: "sheet.png", Hash: "h"}
	frames := buildSpriteSheetFrames(layout, artifact)
	if len(frames) != 12 {
		t.Fatalf("unexpected frame count: %d", len(frames))
	}
	for i, frame := range frames {
		if frame.LogicalFrameIndex != i {
			t.Fatalf("frame %d has logical index %d", i, frame.LogicalFrameIndex)
		}
		if frame.CropRect.Width() != 1024 || frame.CropRect.Height() != 1024 {
			t.Fatalf("frame %d has unexpected crop rect: %dx%d", i, frame.CropRect.Width(), frame.CropRect.Height())
		}
	}
	last := frames[11]
	if last.CropRect.MinX != 3072 || last.CropRect.MinY != 2048 {
		t.Fatalf("unexpected last frame origin: %d,%d", last.CropRect.MinX, last.CropRect.MinY)
	}
}
