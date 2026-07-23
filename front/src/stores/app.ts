// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineStore } from "pinia";
import { ref } from "vue";
import type { Character, Conversation } from "@/types";

export const useAppStore = defineStore("app", () => {
  const characters = ref<Character[]>([]);
  const conversations = ref<Conversation[]>([]);
  const currentCharacter = ref<Character | null>(null);

  const STORAGE_KEY = "uai-user-avatar";
  const avatar = ref(localStorage.getItem(STORAGE_KEY) || "");

  function setAvatar(value: string) {
    avatar.value = value;
    if (value) {
      localStorage.setItem(STORAGE_KEY, value);
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  }

  function removeAvatar() {
    setAvatar("");
  }

  function setCharacters(list: Character[]) {
    characters.value = list;
  }
  function setConversations(list: Conversation[]) {
    conversations.value = list;
  }
  function selectCharacter(c: Character) {
    currentCharacter.value = c;
  }

  const sidebarCollapsed = ref(false);
  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value;
  }

  return {
    characters,
    conversations,
    currentCharacter,
    setCharacters,
    setConversations,
    selectCharacter,
    sidebarCollapsed,
    toggleSidebar,
    avatar,
    setAvatar,
    removeAvatar,
  };
});
