import { ref } from "vue"

const STORAGE_KEY = "uai-user-avatar"
const avatar = ref(localStorage.getItem(STORAGE_KEY) || "")

export function useUserAvatar() {
  function setAvatar(value: string) {
    avatar.value = value
    if (value) {
      localStorage.setItem(STORAGE_KEY, value)
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  }

  return { avatar, setAvatar }
}
