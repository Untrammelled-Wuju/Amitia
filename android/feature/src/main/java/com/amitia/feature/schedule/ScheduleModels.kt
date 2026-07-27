package com.amitia.feature.schedule

import com.amitia.core.designsystem.component.AmitiaStatusType

data class ScheduleItem(
    val id: String,
    val title: String,
    val startTime: String,
    val endTime: String,
    val role: String,
    val status: ScheduleStatus = ScheduleStatus.Upcoming,
    val source: ScheduleSource = ScheduleSource.Manual,
    val channel: String? = null,
    val triggerAction: String? = null,
    val reminder: String? = null,
    val isRoleSchedule: Boolean = false
)

enum class ScheduleStatus(val label: String, val statusType: AmitiaStatusType) {
    Ongoing("进行中", AmitiaStatusType.Running),
    Upcoming("即将开始", AmitiaStatusType.Pending),
    Done("已完成", AmitiaStatusType.Idle),
    Skipped("已跳过", AmitiaStatusType.Degraded),
    Failed("失败", AmitiaStatusType.Failed)
}

enum class ScheduleSource(val label: String) {
    Manual("手动创建"),
    Template("生活模板"),
    Role("角色主动"),
    System("系统触发")
}

enum class CalendarViewMode(val label: String) {
    Month("月视图"), Week("周视图"), Day("日视图")
}

data class CalendarDay(
    val day: Int,
    val isCurrentMonth: Boolean,
    val isToday: Boolean,
    val scheduleCount: Int,
    val hasRoleSchedule: Boolean
)

data class WeekOverview(
    val totalSchedules: Int,
    val completedCount: Int,
    val upcomingCount: Int,
    val roleScheduleCount: Int
)

data class ProactiveMessageWindow(
    val dailyEnabled: Boolean,
    val startTime: String,
    val endTime: String,
    val noonWeightHint: Boolean,
    val frequencyPerDay: Int,
    val minIntervalMinutes: Int,
    val quietHoursEnabled: Boolean,
    val channelStrategy: String
)

data class LifeTemplate(
    val id: String,
    val name: String,
    val description: String,
    val defaultTime: String,
    val category: LifeTemplateCategory,
    val enabled: Boolean,
    val editable: Boolean = true
)

enum class LifeTemplateCategory(val label: String) {
    Daily("日常"), Study("学习"), Work("工作"), Special("特殊")
}

data class StateRule(
    val id: String,
    val name: String,
    val priority: Int,
    val mutuallyExclusiveWith: List<String>,
    val canOverride: Boolean,
    val description: String,
    val enabled: Boolean
)

data class QuietHoursConfig(
    val id: String,
    val name: String,
    val startTime: String,
    val endTime: String,
    val allowEmergency: Boolean,
    val allowedRoles: List<String>,
    val systemNotificationException: Boolean,
    val enabled: Boolean
)

data class ScheduleFormState(
    val title: String = "",
    val date: String = "",
    val startTime: String = "",
    val endTime: String = "",
    val role: String = "",
    val triggerType: ScheduleTriggerType = ScheduleTriggerType.Single,
    val repeatRule: String = "",
    val channel: String = "",
    val reminder: String = "",
    val source: ScheduleSource = ScheduleSource.Manual,
    val timeWindowRandom: Boolean = false,
    val lifeStateTrigger: Boolean = false,
    val roleProactive: Boolean = false
)

enum class ScheduleTriggerType(val label: String) {
    Single("单次"),
    Repeat("重复"),
    TimeWindow("时间窗随机触发"),
    LifeState("生活状态触发"),
    RoleProactive("角色主动消息")
}

object ScheduleMockData {
    val todaySchedules = listOf(
        ScheduleItem(
            id = "1",
            title = "晨间例会",
            startTime = "09:00",
            endTime = "09:30",
            role = "艾米",
            status = ScheduleStatus.Done,
            source = ScheduleSource.Role,
            channel = "微信",
            reminder = "提前 5 分钟"
        ),
        ScheduleItem(
            id = "2",
            title = "项目进度同步",
            startTime = "14:00",
            endTime = "15:00",
            role = "艾米",
            status = ScheduleStatus.Ongoing,
            source = ScheduleSource.Manual,
            channel = "Web",
            triggerAction = "打开项目看板",
            reminder = "提前 15 分钟"
        ),
        ScheduleItem(
            id = "3",
            title = "晚间复盘",
            startTime = "21:00",
            endTime = "21:30",
            role = "艾米",
            status = ScheduleStatus.Upcoming,
            source = ScheduleSource.Template,
            isRoleSchedule = true,
            reminder = "提前 10 分钟"
        )
    )

    val upcomingSchedules = listOf(
        ScheduleItem("4", "明早技术评审", "10:00", "11:00", "艾米", ScheduleStatus.Upcoming, ScheduleSource.Manual, reminder = "明天 09:45"),
        ScheduleItem("5", "周末读书分享", "周六 15:00", "16:00", "艾米", ScheduleStatus.Upcoming, ScheduleSource.Template, isRoleSchedule = true)
    )

    val weekOverview = WeekOverview(
        totalSchedules = 18,
        completedCount = 7,
        upcomingCount = 8,
        roleScheduleCount = 5
    )

    val lifeTemplates = listOf(
        LifeTemplate("t1", "起床", "清晨唤醒与简报", "07:00", LifeTemplateCategory.Daily, false),
        LifeTemplate("t2", "午饭", "午间休息提醒", "12:00", LifeTemplateCategory.Daily, false),
        LifeTemplate("t3", "晚饭", "傍晚用餐提醒", "18:30", LifeTemplateCategory.Daily, false),
        LifeTemplate("t4", "午睡", "午后小憩", "13:00", LifeTemplateCategory.Daily, false),
        LifeTemplate("t5", "睡觉", "夜间入睡准备", "23:00", LifeTemplateCategory.Daily, false),
        LifeTemplate("t6", "上课", "课程时段专注模式", "08:00", LifeTemplateCategory.Study, false),
        LifeTemplate("t7", "上班", "工作时段勿扰", "09:00", LifeTemplateCategory.Work, false),
        LifeTemplate("t8", "考试周", "高专注与作息调整", "08:00", LifeTemplateCategory.Special, false),
        LifeTemplate("t9", "加班", "延长工作时段", "19:00", LifeTemplateCategory.Work, false),
        LifeTemplate("t10", "生病", "降低打扰频率", "全天", LifeTemplateCategory.Special, false),
        LifeTemplate("t11", "图书馆", "深度学习模式", "14:00", LifeTemplateCategory.Study, false)
    )

    val stateRules = listOf(
        StateRule("r1", "上课", 90, listOf("上班"), canOverride = false, "课程进行中保持专注", true),
        StateRule("r2", "上班", 90, listOf("上课"), canOverride = false, "工作时段降低打扰", true),
        StateRule("r3", "生病", 100, emptyList(), canOverride = true, "覆盖普通日程，降低主动消息频率", true),
        StateRule("r4", "考试周", 95, emptyList(), canOverride = true, "覆盖普通日程，启用高专注模式", true),
        StateRule("r5", "午睡", 50, emptyList(), canOverride = false, "短时静音主动消息", false)
    )

    val quietHours = listOf(
        QuietHoursConfig("q1", "夜间休息", "23:00", "07:00", true, listOf("艾米"), true, true),
        QuietHoursConfig("q2", "午休", "13:00", "14:00", false, emptyList(), false, false)
    )

    val proactiveWindow = ProactiveMessageWindow(
        dailyEnabled = true,
        startTime = "10:00",
        endTime = "03:00",
        noonWeightHint = true,
        frequencyPerDay = 6,
        minIntervalMinutes = 90,
        quietHoursEnabled = true,
        channelStrategy = "优先 Web，降级微信"
    )
}
