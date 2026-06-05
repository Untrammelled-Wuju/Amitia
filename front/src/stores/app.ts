import { defineStore } from "pinia"
import { ref } from "vue"
import type { Character, Conversation } from "@/types"

export const useAppStore = defineStore("app", () => {
  const characters = ref<Character[]>([])
  const conversations = ref<Conversation[]>([])
  const currentCharacter = ref<Character | null>(null)

  function setCharacters(list: Character[]) { characters.value = list }
  function setConversations(list: Conversation[]) { conversations.value = list }
  function selectCharacter(c: Character) { currentCharacter.value = c }

  return { characters, conversations, currentCharacter, setCharacters, setConversations, selectCharacter }
})
