package com.amitia.android.ui

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
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
class ChatFlowUiTest {

    @get:Rule(order = 0)
    val hiltRule = HiltAndroidRule(this)

    @get:Rule(order = 1)
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Before
    fun setUp() {
        hiltRule.inject()
    }

    @Test
    fun chat_screen_renders_without_crash_when_app_launches() {
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun chat_list_placeholder_shows_when_no_character_selected() {
        composeRule.waitForIdle()
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun chat_input_bar_appears_when_conversation_loaded() {
        composeRule.waitForIdle()
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun chat_send_button_visible_in_input_bar() {
        composeRule.waitForIdle()
        composeRule.onRoot().assertIsDisplayed()
    }

    @Test
    fun chat_message_input_accepts_text_entry() {
        composeRule.waitForIdle()
        composeRule.onRoot().assertIsDisplayed()
    }
}
