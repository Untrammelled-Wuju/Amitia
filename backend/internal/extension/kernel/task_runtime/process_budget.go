package task_runtime

import (
	"context"
	"errors"
	"sync"
)

var ErrTaskProcessCapacity = errors.New("设备任务进程容量已满，请稍后重试")

type taskProcessBudget struct {
	total       int
	extensions  map[string]int
	definitions map[[2]string]int
}

func (s *TaskRuntimeService) reserveTaskProcess(ctx context.Context, definition *TaskDefinition) (func(), error) {
	if definition == nil || definition.TaskID == "" {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, "任务进程缺少任务身份")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.closed {
		return nil, NewTaskError(ErrTaskRuntimeStartFailed, "设备任务服务已停止")
	}
	extension := definition.ExtensionID
	key := [2]string{extension, definition.TaskID}
	limit := s.config.PerDefinitionMaxConcurrent
	if declared := definition.ResourceLimits.MaxConcurrentTasks; declared > 0 && declared < limit {
		limit = declared
	}
	if s.processBudget.total >= s.config.GlobalMaxConcurrent || s.processBudget.extensions[extension] >= s.config.PerExtensionMaxConcurrent || s.processBudget.definitions[key] >= limit {
		return nil, ErrTaskProcessCapacity
	}
	if s.processBudget.extensions == nil {
		s.processBudget.extensions = make(map[string]int)
		s.processBudget.definitions = make(map[[2]string]int)
	}
	s.processBudget.total++
	s.processBudget.extensions[extension]++
	s.processBudget.definitions[key]++
	var once sync.Once
	release := func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.processBudget.total--
			s.processBudget.extensions[extension]--
			s.processBudget.definitions[key]--
			if s.processBudget.extensions[extension] == 0 {
				delete(s.processBudget.extensions, extension)
			}
			if s.processBudget.definitions[key] == 0 {
				delete(s.processBudget.definitions, key)
			}
		})
	}
	return release, nil
}
