package repair_baseline

import (
	"reflect"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func TestBaseline_Schedule_EnableAcceptsExpectedGeneration(t *testing.T) {
	enableMethod, ok := reflect.TypeOf((*schedule.ScheduleService)(nil)).MethodByName("Enable")
	if !ok {
		t.Fatalf("ScheduleService.Enable method not found")
	}

	if enableMethod.Type.NumIn() < 4 {
		t.Fatalf("ScheduleService.Enable must accept (ctx, scheduleID, expectedGeneration) to fence concurrent generation updates; current param count (incl receiver) = %d, expected >= 4", enableMethod.Type.NumIn())
	}
	lastIn := enableMethod.Type.In(enableMethod.Type.NumIn() - 1)
	if kind := lastIn.Kind(); kind != reflect.Int64 {
		t.Fatalf("ScheduleService.Enable last parameter must be int64 (expectedGeneration), got %v", kind)
	}
}

func TestBaseline_Schedule_DisableAcceptsExpectedGeneration(t *testing.T) {
	m, ok := reflect.TypeOf((*schedule.ScheduleService)(nil)).MethodByName("Disable")
	if !ok {
		t.Fatalf("ScheduleService.Disable method not found")
	}
	if m.Type.NumIn() < 4 {
		t.Fatalf("ScheduleService.Disable must accept (ctx, scheduleID, expectedGeneration); current param count = %d, expected >= 4", m.Type.NumIn())
	}
}

func TestBaseline_Schedule_PauseAcceptsExpectedGeneration(t *testing.T) {
	m, ok := reflect.TypeOf((*schedule.ScheduleService)(nil)).MethodByName("Pause")
	if !ok {
		t.Fatalf("ScheduleService.Pause method not found")
	}
	if m.Type.NumIn() < 4 {
		t.Fatalf("ScheduleService.Pause must accept (ctx, scheduleID, expectedGeneration); current param count = %d, expected >= 4", m.Type.NumIn())
	}
}

func TestBaseline_Schedule_ResumeAcceptsExpectedGeneration(t *testing.T) {
	m, ok := reflect.TypeOf((*schedule.ScheduleService)(nil)).MethodByName("Resume")
	if !ok {
		t.Fatalf("ScheduleService.Resume method not found")
	}
	if m.Type.NumIn() < 4 {
		t.Fatalf("ScheduleService.Resume must accept (ctx, scheduleID, expectedGeneration); current param count = %d, expected >= 4", m.Type.NumIn())
	}
}

func TestBaseline_Schedule_UpdateAcceptsExpectedGeneration(t *testing.T) {
	m, ok := reflect.TypeOf((*schedule.ScheduleService)(nil)).MethodByName("Update")
	if !ok {
		t.Fatalf("ScheduleService.Update method not found")
	}
	if m.Type.NumIn() < 4 {
		t.Fatalf("ScheduleService.Update must accept (ctx, scheduleID, expectedGeneration, definition); current param count = %d, expected >= 4", m.Type.NumIn())
	}
}
