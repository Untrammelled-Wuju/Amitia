package proactive

import (
	"context"
	"log"
)

type ScheduleGeneratorFunc func(date string, characterID string) interface{}

type DueTasksProcessorFunc func(characterID string) interface{}

type BurstTriggerFunc func(characterID string) interface{}

type ScheduleGeneratorContextFunc func(ctx context.Context, date string, characterID string) interface{}

type DueTasksProcessorContextFunc func(ctx context.Context, characterID string) interface{}

type BurstTriggerContextFunc func(ctx context.Context, characterID string) interface{}

var (
	registeredScheduleGenerator ScheduleGeneratorContextFunc
	registeredDueTasksProcessor DueTasksProcessorContextFunc
	registeredBurstTrigger      BurstTriggerContextFunc
)

func RegisterCompanionDispatch(
	scheduleGen ScheduleGeneratorFunc,
	dueProcessor DueTasksProcessorFunc,
	burstTrigger BurstTriggerFunc,
) {
	var scheduleGenWithContext ScheduleGeneratorContextFunc
	if scheduleGen != nil {
		scheduleGenWithContext = func(ctx context.Context, date string, characterID string) interface{} {
			if err := ctx.Err(); err != nil {
				return err
			}
			return scheduleGen(date, characterID)
		}
	}
	var dueProcessorWithContext DueTasksProcessorContextFunc
	if dueProcessor != nil {
		dueProcessorWithContext = func(ctx context.Context, characterID string) interface{} {
			if err := ctx.Err(); err != nil {
				return err
			}
			return dueProcessor(characterID)
		}
	}
	var burstTriggerWithContext BurstTriggerContextFunc
	if burstTrigger != nil {
		burstTriggerWithContext = func(ctx context.Context, characterID string) interface{} {
			if err := ctx.Err(); err != nil {
				return err
			}
			return burstTrigger(characterID)
		}
	}
	RegisterCompanionDispatchContext(scheduleGenWithContext, dueProcessorWithContext, burstTriggerWithContext)
}

func RegisterCompanionDispatchContext(
	scheduleGen ScheduleGeneratorContextFunc,
	dueProcessor DueTasksProcessorContextFunc,
	burstTrigger BurstTriggerContextFunc,
) {
	registeredScheduleGenerator = scheduleGen
	registeredDueTasksProcessor = dueProcessor
	registeredBurstTrigger = burstTrigger
	log.Println("[Dispatch] 已注册 companion 调度函数")
}

func DispatchAll(date string, characterIDs []string) {
	DispatchAllWithContext(context.TODO(), date, characterIDs)
}

func DispatchAllWithContext(ctx context.Context, date string, characterIDs []string) {
	for _, cid := range characterIDs {
		if ctx.Err() != nil {
			return
		}
		if registeredScheduleGenerator != nil {
			registeredScheduleGenerator(ctx, date, cid)
		}
		if registeredBurstTrigger != nil {
			registeredBurstTrigger(ctx, cid)
		}
	}
	log.Printf("[Dispatch] 统一调度完成，角色数=%d", len(characterIDs))
}

func DispatchDueTasks(characterID string) interface{} {
	return DispatchDueTasksWithContext(context.TODO(), characterID)
}

func DispatchDueTasksWithContext(ctx context.Context, characterID string) interface{} {
	if err := ctx.Err(); err != nil {
		return map[string]interface{}{"status": "cancelled", "error": err.Error()}
	}
	if registeredDueTasksProcessor != nil {
		return registeredDueTasksProcessor(ctx, characterID)
	}
	return map[string]interface{}{"status": "no_processor_registered"}
}
