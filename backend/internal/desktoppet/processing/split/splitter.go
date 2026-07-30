package split

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"

	"github.com/u-ai/backend/internal/desktoppet/processing/source"
)

type SplitResult struct {
	Cells []CellArtifact
}

type CellArtifact struct {
	FrameIndex       int
	CellIndex        int
	Row              int
	Column           int
	X                int
	Y                int
	Width            int
	Height           int
	Empty            bool
	Image            *image.NRGBA
	PixelHash        string
	SourceSheetHash  string
}

type Splitter struct{}

func NewSplitter() *Splitter {
	return &Splitter{}
}

func (s *Splitter) Split(sheet *image.NRGBA, layout *source.SpriteSheetLayoutSnapshot, logicalFrameCount int) (*SplitResult, error) {
	if sheet == nil {
		return nil, fmt.Errorf("split: sheet image is nil")
	}
	if layout == nil {
		return nil, fmt.Errorf("split: layout is nil")
	}
	if logicalFrameCount <= 0 {
		return nil, fmt.Errorf("split: logicalFrameCount must be positive, got %d", logicalFrameCount)
	}

	if err := source.ValidateLayout(layout, logicalFrameCount); err != nil {
		return nil, fmt.Errorf("split: validate layout: %w", err)
	}

	cells := source.GenerateCells(layout)
	if len(cells) == 0 {
		return nil, fmt.Errorf("split: no cells generated")
	}

	expectedW, expectedH := source.ComputeExpectedSize(layout)
	if sheet.Bounds().Dx() < expectedW || sheet.Bounds().Dy() < expectedH {
		return nil, fmt.Errorf("split: sheet size %dx%d smaller than expected %dx%d",
			sheet.Bounds().Dx(), sheet.Bounds().Dy(), expectedW, expectedH)
	}

	sourceSheetHash := computeSheetHash(sheet)

	result := &SplitResult{
		Cells: make([]CellArtifact, 0, len(cells)),
	}

	frameSet := make(map[int]bool)
	frameIndexes := make([]int, 0, logicalFrameCount)

	for _, cell := range cells {
		artifact := CellArtifact{
			CellIndex:       cell.CellIndex,
			Row:             cell.Row,
			Column:          cell.Column,
			X:               cell.X,
			Y:               cell.Y,
			Width:           cell.Width,
			Height:          cell.Height,
			SourceSheetHash: sourceSheetHash,
		}

		if cell.Empty || cell.FrameIndex == nil {
			artifact.Empty = true
			artifact.Image = nil
			result.Cells = append(result.Cells, artifact)
			continue
		}

		frameIndex := *cell.FrameIndex
		artifact.FrameIndex = frameIndex

		if cell.Width <= 0 || cell.Height <= 0 {
			return nil, fmt.Errorf("split: cell %d has non-positive dimensions %dx%d",
				cell.CellIndex, cell.Width, cell.Height)
		}

		rect := image.Rect(cell.X, cell.Y, cell.X+cell.Width, cell.Y+cell.Height)
		if !rect.In(sheet.Bounds()) {
			return nil, fmt.Errorf("split: cell %d crop rect %v out of sheet bounds %v",
				cell.CellIndex, rect, sheet.Bounds())
		}

		cropped := cropNRGBA(sheet, rect)
		artifact.Image = cropped
		artifact.PixelHash = computePixelHash(cropped)

		if frameSet[frameIndex] {
			return nil, fmt.Errorf("split: duplicate frame index %d at cell %d",
				frameIndex, cell.CellIndex)
		}
		frameSet[frameIndex] = true
		frameIndexes = append(frameIndexes, frameIndex)

		result.Cells = append(result.Cells, artifact)
	}

	if err := verifyContiguousFrames(frameIndexes, logicalFrameCount); err != nil {
		return nil, err
	}

	return result, nil
}

func cropNRGBA(src *image.NRGBA, rect image.Rectangle) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := 0; y < rect.Dy(); y++ {
		srcRowStart := (rect.Min.Y+y)*src.Stride + rect.Min.X*4
		dstRowStart := y * dst.Stride
		copy(dst.Pix[dstRowStart:dstRowStart+rect.Dx()*4], src.Pix[srcRowStart:srcRowStart+rect.Dx()*4])
	}
	return dst
}

func computePixelHash(img *image.NRGBA) string {
	h := sha256.New()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(img.Bounds().Dx()))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(img.Bounds().Dy()))
	h.Write(buf[:])
	h.Write(img.Pix)
	return hex.EncodeToString(h.Sum(nil))
}

func computeSheetHash(img *image.NRGBA) string {
	h := sha256.New()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(img.Bounds().Dx()))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(img.Bounds().Dy()))
	h.Write(buf[:])
	h.Write(img.Pix)
	return hex.EncodeToString(h.Sum(nil))
}

func verifyContiguousFrames(frameIndexes []int, expectedCount int) error {
	if len(frameIndexes) != expectedCount {
		return fmt.Errorf("split: frame count %d != logicalFrameCount %d",
			len(frameIndexes), expectedCount)
	}
	seen := make(map[int]bool, expectedCount)
	maxIdx := -1
	for _, fi := range frameIndexes {
		if fi < 0 {
			return fmt.Errorf("split: negative frame index %d", fi)
		}
		if seen[fi] {
			return fmt.Errorf("split: duplicate frame index %d", fi)
		}
		seen[fi] = true
		if fi > maxIdx {
			maxIdx = fi
		}
	}
	for i := 0; i < expectedCount; i++ {
		if !seen[i] {
			return fmt.Errorf("split: frame indexes not contiguous 0..%d, missing %d",
				expectedCount-1, i)
		}
	}
	return nil
}
