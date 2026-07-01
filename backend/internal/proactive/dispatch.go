package proactive

import "log"

type ScheduleGeneratorFunc func(date string, characterID string) interface{}

type DueTasksProcessorFunc func(characterID string) interface{}

type BurstTriggerFunc func(characterID string) interface{}

var (
	registeredScheduleGenerator ScheduleGeneratorFunc
	registeredDueTasksProcessor  DueTasksProcessorFunc
	registeredBurstTrigger       BurstTriggerFunc
)

func RegisterCompanionDispatch(
	scheduleGen ScheduleGeneratorFunc,
	dueProcessor DueTasksProcessorFunc,
	burstTrigger BurstTriggerFunc,
) {
	registeredScheduleGenerator = scheduleGen
	registeredDueTasksProcessor = dueProcessor
	registeredBurstTrigger = burstTrigger
	log.Println("[Dispatch] 已注册 companion 调度函数")
}

func DispatchAll(date string, characterIDs []string) {
	for _, cid := range characterIDs {
		if registeredScheduleGenerator != nil {
			registeredScheduleGenerator(date, cid)
		}
		if registeredBurstTrigger != nil {
			registeredBurstTrigger(cid)
		}
	}
	log.Printf("[Dispatch] 统一调度完成，角色数=%d", len(characterIDs))
}

func DispatchDueTasks(characterID string) interface{} {
	if registeredDueTasksProcessor != nil {
		return registeredDueTasksProcessor(characterID)
	}
	return map[string]interface{}{"status": "no_processor_registered"}
}
