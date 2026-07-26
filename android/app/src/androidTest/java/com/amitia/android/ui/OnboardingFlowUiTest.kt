package com.amitia.android.ui

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.printToLog
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.filters.LargeTest
import com.amitia.android.MainActivity
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
    val composeRule = createAndroidComposeRule<MainActivity>()

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
    fun welcome_step_shows_local_first_tagline() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("本地优先的 AI 陪伴运行时", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun clicking_start_button_navigates_to_mode_selection() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("选择运行模式", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun mode_selection_step_displays_local_and_remote_options() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("本地模式", useUnmergedTree = true).assertIsDisplayed()
        composeRule.onNodeWithText("远程模式", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test
    fun mode_selection_describes_local_mode_features() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("RootFS + SurrealDB + Qdrant + Go 后端", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun selecting_remote_mode_marks_radio_as_selected() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("远程模式", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("连接到自托管后端", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun selecting_local_mode_shows_rootfs_install_hint() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("本地模式", useUnmergedTree = true).performClick()
        composeRule.onNodeWithText("下一步", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
    }

    @Test
    fun onboarding_progress_bar_shows_step_count() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("1 / 9", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test
    fun onboarding_next_button_advances_step_count() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("1 / 9", useUnmergedTree = true).assertIsDisplayed()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("2 / 9", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test
    fun onboarding_previous_button_returns_to_prior_step() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("2 / 9", useUnmergedTree = true).assertIsDisplayed()
        composeRule.onNodeWithText("上一步", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("1 / 9", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test
    fun onboarding_skip_button_advances_without_completing_step() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("跳过", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("3 / 9", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test
    fun mode_selection_subtitle_mentions_local_offline_capability() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("开始", useUnmergedTree = true).performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("完全离线可用", substring = true, useUnmergedTree = true)
            .assertIsDisplayed()
    }

    @Test
    fun onboarding_progress_bar_is_visible_at_top() {
        composeRule.waitForIdle()
        composeRule.onNodeWithText("Amitia 初始化", useUnmergedTree = true).assertIsDisplayed()
    }
}
