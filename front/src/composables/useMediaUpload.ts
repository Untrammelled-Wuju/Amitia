import { ref } from "vue"

export function useMediaUpload(
  emit: (e: "image", ...args: any[]) => void,
  emitVideo: (e: "video", ...args: any[]) => void,
  emitRemoveImage: (e: "removeImage") => void,
  emitRemoveVideo: (e: "removeVideo") => void,
) {
  const attachedImage = ref<File | null>(null)
  const attachedImagePreview = ref<string | null>(null)
  const fileInputRef = ref<HTMLInputElement>()
  const videoInputRef = ref<HTMLInputElement>()
  const attachedVideo = ref<File | null>(null)
  const attachedVideoUrl = ref<string | null>(null)
  const uploadingVideo = ref(false)

  function fileToBase64(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(reader.result as string)
      reader.onerror = () => reject(new Error("文件读取失败"))
      reader.readAsDataURL(file)
    })
  }

  function handleImageSelect(e: Event) {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    if (!file) return
    if (!file.type.startsWith("image/")) {
      return
    }
    attachedImage.value = file
    fileToBase64(file).then((dataUrl) => {
      attachedImagePreview.value = dataUrl
      emit("image", file, dataUrl)
    })
    input.value = ""
  }

  function clearImage() {
    attachedImage.value = null
    attachedImagePreview.value = null
    emitRemoveImage("removeImage")
  }

  function handleVideoSelect(e: Event) {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    if (!file) return
    if (!file.type.startsWith("video/")) return
    attachedVideo.value = file
    uploadingVideo.value = true
    const formData = new FormData()
    formData.append("video", file)
    const token = localStorage.getItem("ai-companion-token") || ""
    fetch("/api/video/upload", { method: "POST", headers: { Authorization: "Bearer " + token }, body: formData })
      .then((res) => res.json())
      .then((data) => {
        const videoUrl = data?.data?.videoUrl || data?.videoUrl || ""
        if (videoUrl) {
          attachedVideoUrl.value = videoUrl
          emitVideo("video", file, videoUrl)
        }
      })
      .catch(() => {})
      .finally(() => { uploadingVideo.value = false })
    input.value = ""
  }

  function clearVideo() {
    attachedVideo.value = null
    attachedVideoUrl.value = null
    uploadingVideo.value = false
    emitRemoveVideo("removeVideo")
  }

  return {
    attachedImage,
    attachedImagePreview,
    fileInputRef,
    videoInputRef,
    attachedVideo,
    attachedVideoUrl,
    uploadingVideo,
    handleImageSelect,
    clearImage,
    handleVideoSelect,
    clearVideo,
    fileToBase64,
  }
}
