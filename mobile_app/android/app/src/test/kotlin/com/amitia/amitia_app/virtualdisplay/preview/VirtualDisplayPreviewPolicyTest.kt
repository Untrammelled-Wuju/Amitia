package com.amitia.amitia_app.virtualdisplay.preview

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class VirtualDisplayPreviewPolicyTest {
    @Test
    fun observesCreateOnlyAfterSuccessCanBeInspected() {
        assertFalse(VirtualDisplayPreviewPolicy.shouldObserveBefore("virtual_display.create"))
        assertTrue(VirtualDisplayPreviewPolicy.shouldObserveAfter("virtual_display.create"))
    }

    @Test
    fun observesDisplayActionsBeforeExecution() {
        assertTrue(VirtualDisplayPreviewPolicy.shouldObserveBefore("virtual_display.launch"))
        assertTrue(VirtualDisplayPreviewPolicy.shouldObserveBefore("virtual_display.capture"))
        assertTrue(VirtualDisplayPreviewPolicy.shouldObserveBefore("interaction.click"))
        assertTrue(VirtualDisplayPreviewPolicy.shouldObserveBefore("interaction.visual_locate"))
        assertFalse(VirtualDisplayPreviewPolicy.shouldObserveAfter("interaction.click"))
    }

    @Test
    fun ignoresReadOnlyStatusAndUnrelatedOperations() {
        assertFalse(VirtualDisplayPreviewPolicy.shouldObserveBefore("virtual_display.status"))
        assertFalse(VirtualDisplayPreviewPolicy.shouldObserveBefore("virtual_display.list"))
        assertFalse(VirtualDisplayPreviewPolicy.shouldObserveBefore("interaction.status"))
        assertFalse(VirtualDisplayPreviewPolicy.shouldObserveBefore("notification.status"))
    }

    @Test
    fun handlesReleaseAfterExecution() {
        assertFalse(VirtualDisplayPreviewPolicy.shouldObserveBefore("virtual_display.release"))
        assertTrue(VirtualDisplayPreviewPolicy.shouldObserveAfter("virtual_display.release"))
    }

    @Test
    fun extractsOnlyRealDisplayTargets() {
        assertEquals(7, VirtualDisplayPreviewPolicy.displayId(mapOf("displayId" to 7)))
        assertEquals(0, VirtualDisplayPreviewPolicy.displayId(mapOf("displayId" to 0)))
        assertEquals(-1, VirtualDisplayPreviewPolicy.displayId(emptyMap()))
    }
}
