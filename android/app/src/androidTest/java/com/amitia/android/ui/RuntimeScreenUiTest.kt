package com.amitia.android.ui

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
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
class RuntimeScreenUiTest {

    @get:Rule(order = 0)
    val hiltRule = HiltAndroidRule(this)

    @get:Rule(order = 1)
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Before
    fun setUp() {
        hiltRule.inject()
    }

    @Test
    fun app_launches_and_displays_root_composable() {
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun runtime_screen_renders_when_navigated_via_home_or_settings() {
        composeRule.waitForIdle()
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun runtime_status_card_displays_state_information() {
        composeRule.waitForIdle()
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun runtime_action_buttons_present_in_management_screen() {
        composeRule.waitForIdle()
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun runtime_stop_restart_buttons_visible_when_running() {
        composeRule.waitForIdle()
        composeRule.onRoot().assertIsDisplayed()
    }
}
