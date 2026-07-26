package com.amitia.core.feature

import com.amitia.core.model.CharacterDto
import com.google.common.truth.Truth.assertThat
import org.junit.Test

class CharacterDataSmokeTest {

    @Test
    fun characterDto_preserves_id_and_name() {
        val dto = CharacterDto(id = "c1", name = "Alice")

        assertThat(dto.id).isEqualTo("c1")
        assertThat(dto.name).isEqualTo("Alice")
    }

    @Test
    fun characterDto_default_isCurrent_is_false() {
        val dto = CharacterDto(id = "c1", name = "Alice")

        assertThat(dto.isCurrent).isFalse()
    }

    @Test
    fun characterDto_copy_allows_isCurrent_switch_without_mixing_other_fields() {
        val dto = CharacterDto(
            id = "c1",
            name = "Alice",
            description = "An old friend",
            isCurrent = false
        )

        val switched = dto.copy(isCurrent = true)

        assertThat(switched.id).isEqualTo("c1")
        assertThat(switched.name).isEqualTo("Alice")
        assertThat(switched.description).isEqualTo("An old friend")
        assertThat(switched.isCurrent).isTrue()
        assertThat(dto.isCurrent).isFalse()
    }
}
