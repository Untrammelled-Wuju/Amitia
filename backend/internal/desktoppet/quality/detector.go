// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import "sort"

type DetectorManager struct {
	detectors []Detector
	ordered   []Detector
}

func NewDetectorManager() *DetectorManager {
	return &DetectorManager{}
}

func (m *DetectorManager) Register(d Detector) {
	m.detectors = append(m.detectors, d)
	m.reorder()
}

func (m *DetectorManager) reorder() {
	m.ordered = make([]Detector, len(m.detectors))
	copy(m.ordered, m.detectors)
	sort.SliceStable(m.ordered, func(i, j int) bool {
		return m.ordered[i].Key() < m.ordered[j].Key()
	})
}

func (m *DetectorManager) Ordered() []Detector {
	return m.ordered
}

func (m *DetectorManager) Get(key string) Detector {
	for _, d := range m.ordered {
		if d.Key() == key {
			return d
		}
	}
	return nil
}

var detectorOrder = []string{
	"integrity",
	"alpha",
	"subject",
	"edge",
	"background",
	"stability",
	"identity",
	"motion",
	"duplicate",
	"loop",
	"color",
}

func detectorPriority(key string) int {
	for i, k := range detectorOrder {
		if k == key {
			return i
		}
	}
	return len(detectorOrder)
}

func (m *DetectorManager) RegisterAll(detectors []Detector) {
	m.detectors = append(m.detectors, detectors...)
	m.reorder()
}

func (m *DetectorManager) Count() int {
	return len(m.ordered)
}

type baseDetector struct {
	key     string
	version string
}

func (d *baseDetector) Key() string {
	return d.key
}

func (d *baseDetector) Version() string {
	return d.version
}
