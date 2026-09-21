package conversationstream

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type goldenFixture struct {
	Name                 string              `json:"name"`
	Version              int                 `json:"version"`
	InitialState         goldenExpectedState `json:"initialState"`
	Events               []AgentUIEvent      `json:"events"`
	ExpectedState        goldenExpectedState `json:"expectedState"`
	ExpectedApplyResults []string            `json:"expectedApplyResults"`
}

type goldenExpectedState struct {
	LastEventSequence int64        `json:"lastEventSequence"`
	ActiveTurnID      string       `json:"activeTurnId,omitempty"`
	Turns             []goldenTurn `json:"turns"`
}

type goldenTurn struct {
	TurnID       string        `json:"turnId"`
	TurnSequence int64         `json:"turnSequence"`
	ExecutionID  string        `json:"executionId"`
	Status       string        `json:"status"`
	Blocks       []goldenBlock `json:"blocks"`
}

type goldenBlock struct {
	BlockID       string `json:"blockId"`
	BlockSequence int64  `json:"blockSequence"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	Content       string `json:"content,omitempty"`
	CallID        string `json:"callId,omitempty"`
	Arguments     string `json:"arguments,omitempty"`
	Result        string `json:"result,omitempty"`
}

func TestAgentRuntimeV1GoldenFixtures(t *testing.T) {
	fixtureDir := filepath.Clean(filepath.Join("..", "..", "..", "contracts", "agent-runtime", "v1", "fixtures"))
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read golden fixtures: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		entry := entry
		t.Run(entry.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixtureDir, entry.Name()))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fixture goldenFixture
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			if fixture.Version != ProtocolVersion {
				t.Fatalf("fixture protocol version %d, runtime version %d", fixture.Version, ProtocolVersion)
			}
			actual, results := reduceGoldenFixture(fixture)
			if len(fixture.ExpectedApplyResults) > 0 && !reflect.DeepEqual(results, fixture.ExpectedApplyResults) {
				t.Fatalf("apply results mismatch\nactual: %#v\nwant:   %#v", results, fixture.ExpectedApplyResults)
			}
			if !reflect.DeepEqual(actual, fixture.ExpectedState) {
				actualJSON, _ := json.MarshalIndent(actual, "", "  ")
				wantJSON, _ := json.MarshalIndent(fixture.ExpectedState, "", "  ")
				t.Fatalf("state mismatch\nactual: %s\nwant:   %s", actualJSON, wantJSON)
			}
		})
	}
}

func reduceGoldenFixture(fixture goldenFixture) (goldenExpectedState, []string) {
	turns := make(map[string]goldenTurn, len(fixture.InitialState.Turns))
	var active *ActiveTurnState
	for _, turn := range fixture.InitialState.Turns {
		turns[turn.TurnID] = turn
		if !terminalStatus(turn.Status) {
			candidate := activeTurnFromGolden(turn, fixture.InitialState.LastEventSequence)
			if active == nil || candidate.TurnSequence > active.TurnSequence {
				active = candidate
			}
		}
	}
	lastSequence := fixture.InitialState.LastEventSequence
	seen := map[string]struct{}{}
	results := make([]string, 0, len(fixture.Events))
	for _, event := range fixture.Events {
		if event.Version != ProtocolVersion {
			results = append(results, "unsupported")
			continue
		}
		if event.EventID != "" {
			if _, exists := seen[event.EventID]; exists {
				results = append(results, "duplicate")
				continue
			}
		}
		if event.EventSequence > 0 {
			if lastSequence > 0 && event.EventSequence > lastSequence+1 && !recoveryCheckpoint(event) {
				results = append(results, "gap")
				continue
			}
			if event.EventSequence <= lastSequence {
				if event.EventID != "" {
					seen[event.EventID] = struct{}{}
				}
				results = append(results, "stale")
				continue
			}
		}
		if event.EventID != "" {
			seen[event.EventID] = struct{}{}
		}
		if event.EventSequence > lastSequence {
			lastSequence = event.EventSequence
		}
		if event.TurnID != "" && (active == nil || active.TurnID != event.TurnID) {
			if saved, ok := turns[event.TurnID]; ok && !terminalStatus(saved.Status) {
				active = activeTurnFromGolden(saved, lastSequence)
			} else {
				active = nil
			}
		}
		var terminal bool
		active, terminal = reduceActiveTurn(active, event)
		if active != nil {
			turns[active.TurnID] = goldenFromActive(active)
		}
		if terminal {
			active = nil
		}
		results = append(results, "applied")
	}
	ordered := make([]goldenTurn, 0, len(turns))
	for _, turn := range turns {
		ordered = append(ordered, turn)
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].TurnSequence < ordered[j].TurnSequence })
	activeTurnID := ""
	if active != nil {
		activeTurnID = active.TurnID
	}
	return goldenExpectedState{LastEventSequence: lastSequence, ActiveTurnID: activeTurnID, Turns: ordered}, results
}

func activeTurnFromGolden(turn goldenTurn, lastSequence int64) *ActiveTurnState {
	blocks := make([]BlockState, 0, len(turn.Blocks))
	for _, block := range turn.Blocks {
		payload := map[string]any{}
		if block.Arguments != "" {
			payload["arguments"] = block.Arguments
		}
		if block.Result != "" {
			payload["result"] = block.Result
		}
		blocks = append(blocks, BlockState{BlockID: block.BlockID, BlockSequence: block.BlockSequence, Type: block.Type, Status: block.Status, CallID: block.CallID, Content: block.Content, Payload: payload})
	}
	return &ActiveTurnState{TurnID: turn.TurnID, TurnSequence: turn.TurnSequence, ExecutionID: turn.ExecutionID, Status: turn.Status, LastSequence: lastSequence, Blocks: blocks}
}

func goldenFromActive(turn *ActiveTurnState) goldenTurn {
	blocks := make([]goldenBlock, 0, len(turn.Blocks))
	for _, block := range turn.Blocks {
		item := goldenBlock{BlockID: block.BlockID, BlockSequence: block.BlockSequence, Type: block.Type, Status: block.Status, Content: block.Content, CallID: block.CallID}
		item.Arguments = stringValue(block.Payload["arguments"])
		item.Result = stringValue(block.Payload["result"])
		blocks = append(blocks, item)
	}
	return goldenTurn{TurnID: turn.TurnID, TurnSequence: turn.TurnSequence, ExecutionID: turn.ExecutionID, Status: turn.Status, Blocks: blocks}
}

func terminalStatus(status string) bool {
	return status == "completed" || status == "failed" || status == "interrupted"
}

func recoveryCheckpoint(event AgentUIEvent) bool {
	value, ok := event.Payload["recoveryCheckpoint"].(bool)
	return ok && value
}
