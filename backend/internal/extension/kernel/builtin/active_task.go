package builtin

func BuildActiveTaskExtension(version string) Definition {
	def := buildCharacterFeatureExtension(
		"com.amitia.builtin.active-task",
		version,
		"主动任务",
		"为角色提供时间安排、执行、暂停恢复和历史查询能力。",
		"active-task-core",
		"host.character.active-task",
		"",
		"",
		0,
		[]string{"active_task.read", "active_task.create", "active_task.manage", "active_task.execute"},
		true,
	)
	def.BootstrapRevision = 2
	return def
}
