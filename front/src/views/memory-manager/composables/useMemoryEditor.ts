import { ref } from "vue";

export function useMemoryEditor() {
  const dialogVisible = ref(false);
  const editing = ref(false);
  const editingId = ref("");
  const editingMemory = ref<any | null>(null);

  function showCreate() {
    editing.value = false;
    editingId.value = "";
    editingMemory.value = null;
    dialogVisible.value = true;
  }

  function showEdit(row: any) {
    editing.value = true;
    editingId.value = row?.id || "";
    editingMemory.value = row || null;
    dialogVisible.value = true;
  }

  return {
    dialogVisible,
    editing,
    editingId,
    editingMemory,
    showCreate,
    showEdit,
  };
}
