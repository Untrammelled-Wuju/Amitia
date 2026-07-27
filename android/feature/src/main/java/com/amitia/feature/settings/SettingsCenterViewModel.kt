package com.amitia.feature.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.AmitiaAppearance
import com.amitia.core.designsystem.BlurStrength
import com.amitia.core.designsystem.AmitiaThemeConfig
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class AccountInfo(
    val userName: String = "访客用户",
    val userEmail: String = "",
    val avatarUrl: String? = null,
    val loginMethod: String = "访客模式",
    val plan: String = "免费版",
    val devices: List<DeviceInfo> = listOf(
        DeviceInfo("当前设备", "Android", "在线"),
        DeviceInfo("桌面端", "Windows", "离线")
    )
)

data class DeviceInfo(
    val name: String,
    val platform: String,
    val status: String
)

data class AppearanceSettings(
    val appearance: AmitiaAppearance = AmitiaAppearance.System,
    val dynamicColor: Boolean = false,
    val blurStrength: BlurStrength = BlurStrength.Standard,
    val highContrast: Boolean = false,
    val reduceMotion: Boolean = false,
    val fontScale: Float = 1.0f
) {
    fun toThemeConfig(): AmitiaThemeConfig = AmitiaThemeConfig(
        appearance = appearance,
        dynamicColor = dynamicColor,
        blurStrength = blurStrength,
        highContrast = highContrast,
        reduceMotion = reduceMotion
    )
}

data class NotificationSettings(
    val characterMessages: Boolean = true,
    val proactiveMessages: Boolean = true,
    val schedule: Boolean = true,
    val channelErrors: Boolean = true,
    val modelErrors: Boolean = false,
    val updates: Boolean = true,
    val doNotDisturb: Boolean = false,
    val dndStart: String = "22:00",
    val dndEnd: String = "08:00"
)

data class PrivacySettings(
    val dataStorageLocation: String = "本地",
    val remoteSendEnabled: Boolean = false,
    val logAnonymization: Boolean = true,
    val diagnosticsEnabled: Boolean = false,
    val analyticsEnabled: Boolean = false
)

data class SecuritySettings(
    val appLockEnabled: Boolean = false,
    val biometricEnabled: Boolean = false,
    val sensitiveOperationVerify: Boolean = true,
    val keyStoreStatus: String = "已初始化",
    val computerUseSecurity: Boolean = true,
    val extensionPermissionStrict: Boolean = true
)

data class AppLockSettings(
    val pinEnabled: Boolean = false,
    val biometricEnabled: Boolean = false,
    val lockOnBackground: Boolean = true,
    val lockDelay: String = "立即",
    val hideNotificationContent: Boolean = false
)

data class StorageItem(
    val name: String,
    val size: String,
    val category: String
)

data class StorageInfo(
    val totalUsed: String = "1.2 GB",
    val items: List<StorageItem> = listOf(
        StorageItem("SQLite", "128 MB", "数据库"),
        StorageItem("Qdrant", "456 MB", "向量数据库"),
        StorageItem("SurrealDB", "64 MB", "图数据库"),
        StorageItem("媒体", "320 MB", "媒体文件"),
        StorageItem("缓存", "180 MB", "缓存"),
        StorageItem("日志", "24 MB", "日志")
    )
)

data class BackupItem(
    val id: String,
    val name: String,
    val date: String,
    val size: String,
    val encrypted: Boolean
)

data class BackupState(
    val autoBackup: Boolean = false,
    val encryptionEnabled: Boolean = true,
    val backups: List<BackupItem> = listOf(
        BackupItem("1", "自动备份", "2026-07-26 03:00", "485 MB", true),
        BackupItem("2", "手动备份", "2026-07-20 14:30", "452 MB", true)
    ),
    val isBackingUp: Boolean = false,
    val backupProgress: Float = 0f,
    val isRestoring: Boolean = false,
    val restoreProgress: Float = 0f
)

data class ImportExportItem(
    val name: String,
    val count: String,
    val enabled: Boolean
)

data class RunModeInfo(
    val currentMode: String = "本地模式",
    val isLocal: Boolean = true,
    val remoteAddress: String = "",
    val connectionStatus: String = "未连接"
)

data class RuntimeService(
    val name: String,
    val status: String,
    val version: String,
    val autoStart: Boolean,
    val metrics: Map<String, String> = emptyMap()
)

data class LocalRuntimeInfo(
    val services: List<RuntimeService> = listOf(
        RuntimeService("Linux 环境", "运行中", "proot", true, mapOf("CPU" to "12%", "内存" to "256MB")),
        RuntimeService("Go Backend", "运行中", "0.1.0", true, mapOf("CPU" to "8%", "内存" to "128MB")),
        RuntimeService("SQLite", "运行中", "3.45", true, mapOf("大小" to "128MB")),
        RuntimeService("Qdrant", "运行中", "1.7", true, mapOf("集合" to "3", "向量" to "12K")),
        RuntimeService("SurrealDB", "运行中", "1.5", true, mapOf("表" to "18", "记录" to "4.2K"))
    )
)

data class AutostartBatteryInfo(
    val autostartEnabled: Boolean = false,
    val batteryOptimized: Boolean = false,
    val backgroundRestricted: Boolean = false,
    val persistentNotification: Boolean = true
)

data class NetworkProxyInfo(
    val proxyEnabled: Boolean = false,
    val proxyType: String = "HTTP",
    val proxyAddress: String = "",
    val proxyPort: String = "",
    val wifiOnlyDownload: Boolean = true,
    val dnsStatus: String = "正常",
    val certificateStatus: String = "有效",
    val websocketStatus: String = "已连接"
)

data class PermissionItem(
    val name: String,
    val description: String,
    val granted: Boolean,
    val category: String
)

data class PermissionInfo(
    val systemPermissions: List<PermissionItem> = listOf(
        PermissionItem("通知", "接收消息推送", true, "系统"),
        PermissionItem("存储", "读写本地数据", true, "系统"),
        PermissionItem("麦克风", "语音通话和录音", false, "系统"),
        PermissionItem("相机", "拍照和视频通话", false, "系统"),
        PermissionItem("位置", "地理位置服务", false, "系统")
    ),
    val characterPermissions: List<PermissionItem> = listOf(
        PermissionItem("网络访问", "角色可发起网络请求", true, "角色"),
        PermissionItem("文件读写", "角色可读写文件", true, "角色"),
        PermissionItem("执行命令", "角色可执行系统命令", false, "角色")
    ),
    val extensionPermissions: List<PermissionItem> = listOf(
        PermissionItem("系统通知", "扩展可发送通知", true, "扩展"),
        PermissionItem("后台运行", "扩展可后台运行", false, "扩展")
    )
)

data class LanguageInfo(
    val followSystem: Boolean = true,
    val currentLanguage: String = "zh-CN"
)

data class AccessibilitySettings(
    val highContrast: Boolean = false,
    val reduceMotion: Boolean = false,
    val disableBlur: Boolean = false,
    val largeTouchTarget: Boolean = false,
    val voiceCaption: Boolean = false,
    val graphListMode: Boolean = false
)

data class UpdateInfo(
    val currentVersion: String = "0.1.0",
    val latestVersion: String = "0.1.0",
    val updateAvailable: Boolean = false,
    val updateNotes: String = "",
    val downloadProgress: Float = 0f,
    val isDownloading: Boolean = false,
    val downloadComplete: Boolean = false
)

data class AboutInfo(
    val appName: String = "Amitia",
    val version: String = "0.1.0",
    val buildNumber: String = "1",
    val team: String = "Amitia Team",
    val privacyPolicyUrl: String = "",
    val userAgreementUrl: String = ""
)

data class LicenseItem(
    val name: String,
    val version: String,
    val license: String,
    val url: String
)

data class FeedbackState(
    val issueType: String = "",
    val description: String = "",
    val includeLogs: Boolean = true,
    val contact: String = "",
    val submitted: Boolean = false
)

data class CrashReport(
    val time: String,
    val module: String,
    val safeBoot: Boolean,
    val reportId: String
)

data class CrashRecoveryState(
    val crashes: List<CrashReport> = listOf(
        CrashReport("2026-07-25 14:30", "Runtime", true, "CR-001"),
        CrashReport("2026-07-20 09:15", "Channel", false, "CR-002")
    ),
    val autoSubmit: Boolean = false
)

data class DeveloperOptionsState(
    val enabled: Boolean = false,
    val promptTrace: Boolean = false,
    val networkLog: Boolean = false,
    val runtimeConsole: Boolean = false,
    val uiDebug: Boolean = false,
    val experimentalFeatures: Boolean = false
)

data class SettingsCenterUiState(
    val account: AccountInfo = AccountInfo(),
    val appearance: AppearanceSettings = AppearanceSettings(),
    val notifications: NotificationSettings = NotificationSettings(),
    val privacy: PrivacySettings = PrivacySettings(),
    val security: SecuritySettings = SecuritySettings(),
    val appLock: AppLockSettings = AppLockSettings(),
    val storage: StorageInfo = StorageInfo(),
    val backup: BackupState = BackupState(),
    val importExport: List<ImportExportItem> = listOf(
        ImportExportItem("角色", "12 个", true),
        ImportExportItem("对话", "1,248 条", true),
        ImportExportItem("记忆", "3,562 条", true),
        ImportExportItem("世界书", "8 本", true),
        ImportExportItem("扩展", "5 个", true),
        ImportExportItem("完整数据包", "全部", true)
    ),
    val runMode: RunModeInfo = RunModeInfo(),
    val localRuntime: LocalRuntimeInfo = LocalRuntimeInfo(),
    val autostartBattery: AutostartBatteryInfo = AutostartBatteryInfo(),
    val networkProxy: NetworkProxyInfo = NetworkProxyInfo(),
    val permissions: PermissionInfo = PermissionInfo(),
    val language: LanguageInfo = LanguageInfo(),
    val accessibility: AccessibilitySettings = AccessibilitySettings(),
    val update: UpdateInfo = UpdateInfo(),
    val about: AboutInfo = AboutInfo(),
    val licenses: List<LicenseItem> = listOf(
        LicenseItem("Jetpack Compose", "1.7", "Apache 2.0", ""),
        LicenseItem("Kotlin", "2.0", "Apache 2.0", ""),
        LicenseItem("Hilt", "2.51", "Apache 2.0", ""),
        LicenseItem("Qdrant", "1.7", "Apache 2.0", ""),
        LicenseItem("SurrealDB", "1.5", "BSL", ""),
        LicenseItem("Material 3", "1.3", "Apache 2.0", "")
    ),
    val feedback: FeedbackState = FeedbackState(),
    val crashRecovery: CrashRecoveryState = CrashRecoveryState(),
    val developer: DeveloperOptionsState = DeveloperOptionsState(),
    val isLoading: Boolean = false,
    val error: String? = null
)

@HiltViewModel
class SettingsCenterViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow(SettingsCenterUiState())
    val state: StateFlow<SettingsCenterUiState> = _state.asStateFlow()

    fun updateAppearance(settings: AppearanceSettings) {
        _state.value = _state.value.copy(appearance = settings)
    }

    fun updateNotifications(settings: NotificationSettings) {
        _state.value = _state.value.copy(notifications = settings)
    }

    fun updatePrivacy(settings: PrivacySettings) {
        _state.value = _state.value.copy(privacy = settings)
    }

    fun updateSecurity(settings: SecuritySettings) {
        _state.value = _state.value.copy(security = settings)
    }

    fun updateAppLock(settings: AppLockSettings) {
        _state.value = _state.value.copy(appLock = settings)
    }

    fun updateAutostartBattery(settings: AutostartBatteryInfo) {
        _state.value = _state.value.copy(autostartBattery = settings)
    }

    fun updateNetworkProxy(settings: NetworkProxyInfo) {
        _state.value = _state.value.copy(networkProxy = settings)
    }

    fun updateLanguage(settings: LanguageInfo) {
        _state.value = _state.value.copy(language = settings)
    }

    fun updateAccessibility(settings: AccessibilitySettings) {
        _state.value = _state.value.copy(accessibility = settings)
    }

    fun updateDeveloper(settings: DeveloperOptionsState) {
        _state.value = _state.value.copy(developer = settings)
    }

    fun updateCrashRecovery(state: CrashRecoveryState) {
        _state.value = _state.value.copy(crashRecovery = state)
    }

    fun updateFeedback(state: FeedbackState) {
        _state.value = _state.value.copy(feedback = state)
    }

    fun updateAutoBackup(enabled: Boolean) {
        _state.value = _state.value.copy(
            backup = _state.value.backup.copy(autoBackup = enabled)
        )
    }

    fun startBackup() {
        _state.value = _state.value.copy(backup = _state.value.backup.copy(isBackingUp = true, backupProgress = 0f))
        viewModelScope.launch {
            for (i in 1..10) {
                kotlinx.coroutines.delay(200)
                _state.value = _state.value.copy(
                    backup = _state.value.backup.copy(backupProgress = i * 0.1f)
                )
            }
            _state.value = _state.value.copy(
                backup = _state.value.backup.copy(
                    isBackingUp = false,
                    backupProgress = 1f,
                    backups = listOf(
                        BackupItem("new", "手动备份", "刚刚", "488 MB", true)
                    ) + _state.value.backup.backups
                )
            )
        }
    }

    fun startRestore(backupId: String) {
        _state.value = _state.value.copy(backup = _state.value.backup.copy(isRestoring = true, restoreProgress = 0f))
        viewModelScope.launch {
            for (i in 1..10) {
                kotlinx.coroutines.delay(200)
                _state.value = _state.value.copy(
                    backup = _state.value.backup.copy(restoreProgress = i * 0.1f)
                )
            }
            _state.value = _state.value.copy(
                backup = _state.value.backup.copy(isRestoring = false, restoreProgress = 1f)
            )
        }
    }

    fun clearCache() {
        val updated = _state.value.storage.copy(
            totalUsed = "1.0 GB",
            items = _state.value.storage.items.map {
                if (it.name == "缓存") it.copy(size = "0 MB") else it
            }
        )
        _state.value = _state.value.copy(storage = updated)
    }

    fun checkUpdate() {
        _state.value = _state.value.copy(update = _state.value.update.copy(isDownloading = true, downloadProgress = 0f))
        viewModelScope.launch {
            for (i in 1..10) {
                kotlinx.coroutines.delay(300)
                _state.value = _state.value.copy(
                    update = _state.value.update.copy(downloadProgress = i * 0.1f)
                )
            }
            _state.value = _state.value.copy(
                update = _state.value.update.copy(
                    isDownloading = false,
                    downloadComplete = true,
                    downloadProgress = 1f
                )
            )
        }
    }
}
