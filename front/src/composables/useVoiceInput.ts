// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, onUnmounted } from "vue"

const VOICE_END_DELAY = 400
const SLIDE_TEXT_THRESHOLD = 60
const SLIDE_CANCEL_THRESHOLD = 120

export function useVoiceInput(
  onVoiceText: (text: string) => void,
  onVoiceAudio: (blob: Blob, transcript?: string, duration?: number) => void,
  isDisabled: () => boolean,
  isSending: () => boolean,
) {
  const voiceMode = ref(false)
  const holding = ref(false)
  const listening = ref(false)
  const slideZone = ref<"none" | "text" | "cancel">("none")
  const touchStartY = ref(0)
  const lastTranscript = ref("")

  let recognition: any = null
  let mediaRecorder: MediaRecorder | null = null
  let audioChunks: Blob[] = []
  let onRecordingComplete: (() => void) | null = null
  let recordingStartTime = 0

  function onGlobalMouseUp() {
    document.removeEventListener("mouseup", onGlobalMouseUp)
    endHold()
  }

  function startHold(e: TouchEvent | MouseEvent) {
    if (isDisabled() || isSending()) return
    holding.value = true
    slideZone.value = "none"
    lastTranscript.value = ""
    if ("touches" in e) {
      touchStartY.value = e.touches[0].clientY
    } else {
      touchStartY.value = e.clientY
    }
    startRecording()
    startListening()
    document.addEventListener("mouseup", onGlobalMouseUp)
  }

  function onTouchMove(e: TouchEvent) {
    if (!holding.value) return
    const currentY = e.touches[0].clientY
    const deltaY = touchStartY.value - currentY
    if (deltaY > SLIDE_CANCEL_THRESHOLD) {
      slideZone.value = "cancel"
    } else if (deltaY > SLIDE_TEXT_THRESHOLD) {
      slideZone.value = "text"
    } else {
      slideZone.value = "none"
    }
  }

  function endHold() {
    if (!holding.value) return
    const zone = slideZone.value
    holding.value = false
    slideZone.value = "none"

    setTimeout(() => {
      document.removeEventListener("mouseup", onGlobalMouseUp)
      stopListening()

      if (zone === "cancel") {
        stopRecording()
        audioChunks = []
        return
      }

      if (zone === "text") {
        stopRecording()
        audioChunks = []
        if (lastTranscript.value) {
          onVoiceText(lastTranscript.value)
        }
        voiceMode.value = false
        return
      }

      onRecordingComplete = () => {
        try {
          if (audioChunks.length > 0) {
            const blob = new Blob(audioChunks, { type: "audio/webm" })
            const transcript = lastTranscript.value || undefined
            audioChunks = []
            const duration = Math.round((Date.now() - recordingStartTime) / 1000)
            try { onVoiceAudio(blob, transcript, duration) } catch (e) { console.error("[Voice] emit error:", e) }
          }
          voiceMode.value = false
        } catch (e) {
          console.error("[Voice] onRecordingComplete error:", e)
          audioChunks = []
          voiceMode.value = false
        }
      }
      stopRecording()
    }, VOICE_END_DELAY)
  }

  function startRecording() {
    navigator.mediaDevices.getUserMedia({ audio: true }).then((stream) => {
      recordingStartTime = Date.now()

      const mimeType = MediaRecorder.isTypeSupported("audio/webm;codecs=opus")
        ? "audio/webm;codecs=opus"
        : "audio/webm"
      mediaRecorder = new MediaRecorder(stream, { mimeType })
      mediaRecorder.ondataavailable = (e: BlobEvent) => {
        if (e.data.size > 0) audioChunks.push(e.data)
      }
      mediaRecorder.onstop = () => {
        stream.getTracks().forEach((t) => t.stop())
        if (onRecordingComplete) {
          onRecordingComplete()
          onRecordingComplete = null
        }
      }
      mediaRecorder.start()
    }).catch(() => {
      holding.value = false
    })
  }

  function stopRecording() {
    if (mediaRecorder && mediaRecorder.state !== "inactive") {
      mediaRecorder.stop()
    }
    mediaRecorder = null
  }

  function startListening() {
    const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition
    if (!SpeechRecognition) {
      return
    }
    if (recognition) {
      try { recognition.stop() } catch {}
      recognition = null
    }
    recognition = new SpeechRecognition()
    recognition.lang = "zh-CN"
    recognition.interimResults = true
    recognition.maxAlternatives = 1
    recognition.continuous = false

    recognition.onstart = () => { listening.value = true }
    recognition.onresult = (event: any) => {
      let finalTranscript = ""
      for (let i = event.resultIndex; i < event.results.length; i++) {
        if (event.results[i].isFinal) {
          finalTranscript += event.results[i][0].transcript
        }
      }
      if (finalTranscript) {
        lastTranscript.value = finalTranscript
      } else if (event.results.length > 0) {
        lastTranscript.value = event.results[event.results.length - 1][0].transcript
      }
    }
    recognition.onerror = () => {
      listening.value = false
    }
    recognition.onend = () => { listening.value = false }

    try {
      recognition.start()
    } catch {
      listening.value = false
    }
  }

  function stopListening() {
    if (recognition) {
      try { recognition.stop() } catch {}
      recognition = null
    }
    listening.value = false
  }

  onUnmounted(() => {
    stopListening()
    stopRecording()
  })

  return {
    voiceMode,
    holding,
    listening,
    slideZone,
    startHold,
    endHold,
    onTouchMove,
  }
}
