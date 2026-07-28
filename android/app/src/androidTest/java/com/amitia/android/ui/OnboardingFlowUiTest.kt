package com.amitia.android.ui

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.printToLog
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.filters.LargeTest
import com.amitia.android.onboarding.OnboardingActivity
import dagger.hilt.android.testing.HiltAndroidRule
import dagger.hilt.android.testing.HiltAndroidTest
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@HiltAndroidTest
@RunWith(AndroidJUnit4::class)
@LargeTest
class OnboardingFlowUiTest {

    @get:Rule(order = 0)
    val hiltRule = HiltAndroidRule(this)

    @get:Rule(order = 1)
    val composeRule = createAndroidComposeRule<OnboardingActivity>()

    @Before
    fun setUp() {
        hiltRule.inject()
    }

    @Test
    fun app_launches_without_crash_and_renders_root() {
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun welcome_step_shows_amitia_branding_text() {
        composeRule.onRoot().printToLog("OnboardingFlowUiTest")
        composeRule.waitForIdle()
        composeRule.onNodeWithText("Amitia", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun welcome_step_shows_companion_tagline() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("你的专属 AI 伙伴", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun clicking_start_button_navigates_to_mode_selection() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("选择运行方式", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun mode_selection_step_displays_local_and_remote_options() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("本地运行", useUnmergedTree = true).assertIsDisplayed()
        composeRule.onNodeWithText("远程连接", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test
    fun mode_selection_describes_local_mode_features() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("数据优先保存在本机", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun selecting_remote_mode_shows_remote_description() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("远程连接", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("连接已有 Amitia 服务端", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun selecting_local_mode_and_proceeding_shows_environment_step() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("本地运行", useUnmergedTree = true).performClick()
        composeRule.onNodeWithText("下一步", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("准备运行环境", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun onboarding_step_label_shows_correct_count() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("1 / 6", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test
    fun onboarding_next_button_advances_step_count() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("1 / 6", useUnmergedTree = true).assertIsDisplayed()
        composeRule.onNodeWithText("本地运行", useUnmergedTree = true).performClick()
        composeRule.onNodeWithText("下一步", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("2 / 6", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test
    fun mode_selection_subtitle_mentions_settings_switch() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("稍后在设置中切换", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun welcome_step_shows_start_button() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始设置", useUnmergedTree = true).assertIsDisplayed()
    }
}
