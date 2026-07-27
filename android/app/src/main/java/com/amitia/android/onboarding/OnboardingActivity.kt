package com.amitia.android.onboarding

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.amitia.android.MainActivity
import com.amitia.android.bootstrap.BootstrapActivity
import com.amitia.android.navigation.AmitiaRoutes
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaSpacing
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class OnboardingActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            AmitiaTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    OnboardingNavHost(
                        onComplete = { completeOnboarding() }
                    )
                }
            }
        }
    }

    private fun completeOnboarding() {
        val prefs = getSharedPreferences(BootstrapActivity.PREFS_NAME, MODE_PRIVATE)
        BootstrapActivity.markOnboardingCompleted(prefs)
        startActivity(Intent(this, MainActivity::class.java))
        finish()
    }
}

private data class OnboardingStep(
    val route: String,
    val title: String,
    val subtitle: String
)

private val onboardingSteps: List<OnboardingStep> = listOf(
    OnboardingStep(AmitiaRoutes.Onboarding.WELCOME, "欢迎", "欢迎使用 Amitia，你的专属 AI 伙伴"),
    OnboardingStep(AmitiaRoutes.Onboarding.ENVIRONMENT_CHECK, "环境检查", "正在检查设备环境与系统兼容性"),
    OnboardingStep(AmitiaRoutes.Onboarding.MODE_SELECT, "模式选择", "选择本地运行或远程连接模式"),
    OnboardingStep(AmitiaRoutes.Onboarding.LOCAL_INSTALL, "本地安装", "下载并安装本地运行时核心"),
    OnboardingStep(AmitiaRoutes.Onboarding.REMOTE_CONNECT, "远程连接", "配置远程服务端连接参数"),
    OnboardingStep(AmitiaRoutes.Onboarding.ACCOUNT_ENTRY, "账号入口", "登录或注册你的账号"),
    OnboardingStep(AmitiaRoutes.Onboarding.REGISTER, "注册", "创建新的 Amitia 账号"),
    OnboardingStep(AmitiaRoutes.Onboarding.LOGIN, "登录", "使用已有账号登录"),
    OnboardingStep(AmitiaRoutes.Onboarding.PERMISSIONS, "权限授予", "授予通知、麦克风、存储等必要权限"),
    OnboardingStep(AmitiaRoutes.Onboarding.MODEL_TEXT, "文本模型", "配置对话使用的语言模型"),
    OnboardingStep(AmitiaRoutes.Onboarding.MODEL_VISION, "视觉模型", "配置图像理解与生成模型"),
    OnboardingStep(AmitiaRoutes.Onboarding.MODEL_VOICE, "语音模型", "配置语音合成与识别模型"),
    OnboardingStep(AmitiaRoutes.Onboarding.MODEL_VECTOR, "向量模型", "配置嵌入向量模型用于记忆检索"),
    OnboardingStep(AmitiaRoutes.Onboarding.CHARACTER_APPEARANCE, "角色形象", "设定角色的外观形象"),
    OnboardingStep(AmitiaRoutes.Onboarding.CHARACTER_NAME, "角色名称", "为你的角色取一个名字"),
    OnboardingStep(AmitiaRoutes.Onboarding.CHARACTER_IDENTITY, "角色身份", "设定角色的身份与背景"),
    OnboardingStep(AmitiaRoutes.Onboarding.CHARACTER_PERSONALITY, "角色性格", "塑造角色的性格特征"),
    OnboardingStep(AmitiaRoutes.Onboarding.INITIAL_MEMORY_1, "初始记忆 一", "为角色写入第一段初始记忆"),
    OnboardingStep(AmitiaRoutes.Onboarding.INITIAL_MEMORY_2, "初始记忆 二", "为角色写入第二段初始记忆"),
    OnboardingStep(AmitiaRoutes.Onboarding.INITIAL_MEMORY_3, "初始记忆 三", "为角色写入第三段初始记忆"),
    OnboardingStep(AmitiaRoutes.Onboarding.SETUP_SUMMARY, "设置摘要", "确认所有配置项无误"),
    OnboardingStep(AmitiaRoutes.Onboarding.CHARACTER_COMPLETE, "角色完成", "角色创建即将完成"),
    OnboardingStep(AmitiaRoutes.Onboarding.ENTER_AMITIA, "进入 Amitia", "一切就绪，开始你的旅程")
)

@Composable
private fun OnboardingNavHost(
    onComplete: () -> Unit
) {
    val navController = rememberNavController()
    val total = onboardingSteps.size

    Box(modifier = Modifier.fillMaxSize()) {
        NavHost(
            navController = navController,
            startDestination = AmitiaRoutes.Onboarding.WELCOME,
            modifier = Modifier.fillMaxSize()
        ) {
            onboardingSteps.forEachIndexed { index, step ->
                composable(step.route) {
                    val isLast = index == total - 1
                    OnboardingStepScreen(
                        title = step.title,
                        subtitle = step.subtitle,
                        stepIndex = index,
                        total = total,
                        onBack = if (index == 0) null else {
                            { navController.popBackStack() }
                        },
                        onNext = {
                            if (isLast) {
                                onComplete()
                            } else {
                                navController.navigate(onboardingSteps[index + 1].route) {
                                    launchSingleTop = true
                                }
                            }
                        }
                    )
                }
            }
        }
        OnboardingProgressLine(
            navController = navController,
            total = total,
            modifier = Modifier.align(Alignment.TopCenter)
        )
    }
}

@Composable
private fun OnboardingProgressLine(
    navController: NavHostController,
    total: Int,
    modifier: Modifier = Modifier
) {
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = backStackEntry?.destination?.route
    val index = onboardingSteps.indexOfFirst { it.route == currentRoute }.coerceAtLeast(0)
    val progress = if (total == 0) 0f else (index + 1f) / total
    LinearProgressIndicator(
        progress = { progress },
        modifier = modifier
            .fillMaxWidth()
            .height(2.dp),
        color = MaterialTheme.colorScheme.primary,
        trackColor = MaterialTheme.colorScheme.surfaceVariant
    )
}

@Composable
private fun OnboardingStepScreen(
    title: String,
    subtitle: String,
    stepIndex: Int,
    total: Int,
    onBack: (() -> Unit)?,
    onNext: () -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(AmitiaSpacing.Xxl),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text(
            text = "第 ${stepIndex + 1} / $total 步",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = title,
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onSurface,
            textAlign = TextAlign.Center
        )
        Text(
            text = subtitle,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center
        )
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = AmitiaSpacing.Xxl),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically
        ) {
            if (onBack != null) {
                OutlinedButton(
                    onClick = onBack,
                    modifier = Modifier.weight(1f)
                ) {
                    Text("上一步")
                }
            }
            Button(
                onClick = onNext,
                modifier = Modifier.weight(1f)
            ) {
                Text(if (stepIndex == total - 1) "进入 Amitia" else "下一步")
            }
        }
    }
}
