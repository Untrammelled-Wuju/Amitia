package com.amitia.feature.character

import app.cash.turbine.test
import com.amitia.core.model.CharacterCreateRequest
import com.amitia.core.model.CharacterDto
import com.amitia.core.model.CharacterUpdateRequest
import com.amitia.core.repository.CharacterRepository
import com.google.common.truth.Truth.assertThat
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class CharacterViewModelTest {

    private val repository: CharacterRepository = mockk(relaxed = true)
    private val testDispatcher = UnconfinedTestDispatcher()
    private lateinit var viewModel: CharacterViewModel

    private fun character(id: String, name: String, isCurrent: Boolean = false) = CharacterDto(
        id = id,
        name = name,
        isCurrent = isCurrent
    )

    @Before
    fun setUp() {
        Dispatchers.setMain(testDispatcher)
        coEvery { repository.list(any(), any()) } returns emptyList()
        coEvery { repository.getCurrent() } returns character("char-current", "Default", isCurrent = true)
        viewModel = CharacterViewModel(repository)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    @Test
    fun init_loads_characters_and_current_id() = runTest {
        val characters = listOf(
            character("c1", "Alice", isCurrent = true),
            character("c2", "Bob")
        )
        coEvery { repository.list(any(), any()) } returns characters
        coEvery { repository.getCurrent() } returns character("c1", "Alice", isCurrent = true)

        val vm = CharacterViewModel(repository)

        vm.state.test {
            val state = awaitItem()
            assertThat(state.characters).hasSize(2)
            assertThat(state.currentCharacterId).isEqualTo("c1")
            assertThat(state.loading).isFalse()
        }
    }

    @Test
    fun switchCharacter_updates_current_id_and_marks_isCurrent_flag() = runTest {
        val initial = listOf(
            character("c1", "Alice", isCurrent = true),
            character("c2", "Bob")
        )
        coEvery { repository.list(any(), any()) } returns initial
        coEvery { repository.getCurrent() } returns initial[0]
        coEvery { repository.switchCurrent("c2") } returns character("c2", "Bob", isCurrent = true)

        viewModel.listCharacters()
        advanceUntilIdle()

        viewModel.switchCharacter("c2")
        advanceUntilIdle()

        val state = viewModel.state.value
        assertThat(state.currentCharacterId).isEqualTo("c2")
        assertThat(state.characters.first { it.id == "c2" }.isCurrent).isTrue()
        assertThat(state.characters.first { it.id == "c1" }.isCurrent).isFalse()
    }

    @Test
    fun switchCharacter_does_not_mix_data_with_other_character() = runTest {
        val characters = listOf(
            character("c1", "Alice", isCurrent = true),
            character("c2", "Bob"),
            character("c3", "Carol")
        )
        coEvery { repository.list(any(), any()) } returns characters
        coEvery { repository.getCurrent() } returns characters[0]
        coEvery { repository.switchCurrent("c3") } returns character("c3", "Carol", isCurrent = true)

        viewModel.listCharacters()
        advanceUntilIdle()

        viewModel.switchCharacter("c3")
        advanceUntilIdle()

        val state = viewModel.state.value
        assertThat(state.currentCharacterId).isEqualTo("c3")
        assertThat(state.characters.map { it.id }).containsExactly("c1", "c2", "c3")
        assertThat(state.characters.first { it.id == "c3" }.name).isEqualTo("Carol")
    }

    @Test
    fun switchCharacter_sets_error_when_repository_fails() = runTest {
        coEvery { repository.switchCurrent("c-bad") } throws RuntimeException("switch failed")

        viewModel.switchCharacter("c-bad")
        advanceUntilIdle()

        val state = viewModel.state.value
        assertThat(state.error).contains("switch failed")
        assertThat(state.loading).isFalse()
    }

    @Test
    fun loadDetail_loads_character_into_detail_state() = runTest {
        val detail = character("c1", "Alice", isCurrent = true).copy(description = "An old friend")
        coEvery { repository.get("c1") } returns detail

        viewModel.loadDetail("c1")
        advanceUntilIdle()

        val detailState = viewModel.detailState.value
        assertThat(detailState.character).isNotNull()
        assertThat(detailState.character?.description).isEqualTo("An old friend")
        assertThat(detailState.loading).isFalse()
    }

    @Test
    fun createCharacter_appends_to_list_and_switches_current() = runTest {
        val created = character("c-new", "Zoe")
        coEvery { repository.create(any()) } returns created
        coEvery { repository.switchCurrent("c-new") } returns created.copy(isCurrent = true)
        var createdId: String? = null

        viewModel.createCharacter(
            request = CharacterCreateRequest(name = "Zoe"),
            onCreated = { createdId = it }
        )
        advanceUntilIdle()

        val state = viewModel.state.value
        assertThat(state.characters.map { it.id }).contains("c-new")
        assertThat(state.currentCharacterId).isEqualTo("c-new")
        assertThat(createdId).isEqualTo("c-new")
    }

    @Test
    fun updateCharacter_replaces_existing_entry_in_list() = runTest {
        val original = character("c1", "Alice", isCurrent = true)
        coEvery { repository.list(any(), any()) } returns listOf(original)
        coEvery { repository.getCurrent() } returns original
        val updated = original.copy(name = "Alice Updated")
        coEvery { repository.update("c1", any()) } returns updated

        val vm = CharacterViewModel(repository)
        advanceUntilIdle()

        var updateCallback = false
        vm.updateCharacter("c1", CharacterUpdateRequest(name = "Alice Updated")) {
            updateCallback = true
        }
        advanceUntilIdle()

        val state = vm.state.value
        assertThat(state.characters.first { it.id == "c1" }.name).isEqualTo("Alice Updated")
        assertThat(updateCallback).isTrue()
    }

    @Test
    fun deleteCharacter_removes_from_list_and_clears_pending_delete() = runTest {
        val characters = listOf(
            character("c1", "Alice", isCurrent = true),
            character("c2", "Bob")
        )
        coEvery { repository.list(any(), any()) } returns characters
        coEvery { repository.getCurrent() } returns characters[0]
        coEvery { repository.delete("c1") } returns Unit

        viewModel.confirmDelete("c1")
        assertThat(viewModel.state.value.pendingDeleteId).isEqualTo("c1")

        var deletedCallback = false
        viewModel.deleteCharacter("c1") { deletedCallback = true }
        advanceUntilIdle()

        val state = viewModel.state.value
        assertThat(state.characters.map { it.id }).doesNotContain("c1")
        assertThat(state.pendingDeleteId).isNull()
        assertThat(deletedCallback).isTrue()
    }

    @Test
    fun confirmDelete_sets_pendingDeleteId() = runTest {
        viewModel.confirmDelete("c-target")

        assertThat(viewModel.state.value.pendingDeleteId).isEqualTo("c-target")
    }

    @Test
    fun dismissDelete_clears_pendingDeleteId() = runTest {
        viewModel.confirmDelete("c-target")
        viewModel.dismissDelete()

        assertThat(viewModel.state.value.pendingDeleteId).isNull()
    }

    @Test
    fun consumeError_clears_error_state() = runTest {
        coEvery { repository.switchCurrent(any()) } throws RuntimeException("boom")
        viewModel.switchCharacter("c-bad")
        advanceUntilIdle()
        assertThat(viewModel.state.value.error).isNotNull()

        viewModel.consumeError()

        assertThat(viewModel.state.value.error).isNull()
        assertThat(viewModel.detailState.value.error).isNull()
    }

    @Test
    fun listCharacters_invokes_repository_with_default_pagination() = runTest {
        coEvery { repository.list(any(), any()) } returns emptyList()
        coEvery { repository.getCurrent() } returns character("c1", "A", isCurrent = true)

        viewModel.listCharacters()
        advanceUntilIdle()

        coVerify { repository.list(page = 1, pageSize = 50) }
    }
}
