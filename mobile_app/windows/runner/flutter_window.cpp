#include "flutter_window.h"

#include <mmsystem.h>
#include <shellapi.h>
#include <strsafe.h>

#include <atomic>
#include <chrono>
#include <optional>
#include <string>
#include <thread>

#include "flutter/generated_plugin_registrant.h"
#include "resource.h"
#include "utils.h"

namespace {

using flutter::EncodableMap;
using flutter::EncodableValue;

std::string GetString(const EncodableMap& map, const char* key) {
  const auto it = map.find(EncodableValue(key));
  if (it == map.end()) {
    return std::string();
  }
  if (const auto* value = std::get_if<std::string>(&it->second)) {
    return *value;
  }
  return std::string();
}

bool GetBool(const EncodableMap& map, const char* key, bool fallback = false) {
  const auto it = map.find(EncodableValue(key));
  if (it == map.end()) {
    return fallback;
  }
  if (const auto* value = std::get_if<bool>(&it->second)) {
    return *value;
  }
  return fallback;
}

const EncodableMap* GetMap(const EncodableMap& map, const char* key) {
  const auto it = map.find(EncodableValue(key));
  if (it == map.end()) {
    return nullptr;
  }
  return std::get_if<EncodableMap>(&it->second);
}

std::wstring WideFromUtf8(const std::string& value) {
  if (value.empty()) {
    return std::wstring();
  }
  const int required = MultiByteToWideChar(
      CP_UTF8, MB_ERR_INVALID_CHARS, value.data(), static_cast<int>(value.size()),
      nullptr, 0);
  if (required <= 0) {
    return std::wstring();
  }
  std::wstring result(static_cast<size_t>(required), L'\0');
  const int converted = MultiByteToWideChar(
      CP_UTF8, MB_ERR_INVALID_CHARS, value.data(), static_cast<int>(value.size()),
      result.data(), required);
  if (converted <= 0) {
    return std::wstring();
  }
  return result;
}

EncodableValue SuccessResponse(const std::string& request_id,
                               EncodableMap payload = EncodableMap()) {
  EncodableMap response;
  response[EncodableValue("protocolVersion")] = EncodableValue(1);
  response[EncodableValue("requestId")] = EncodableValue(request_id);
  response[EncodableValue("status")] = EncodableValue("success");
  response[EncodableValue("result")] = EncodableValue(std::move(payload));
  response[EncodableValue("error")] = EncodableValue();
  return EncodableValue(std::move(response));
}

EncodableValue ErrorResponse(const std::string& request_id,
                             const std::string& code,
                             const std::string& message) {
  EncodableMap error;
  error[EncodableValue("code")] = EncodableValue(code);
  error[EncodableValue("message")] = EncodableValue(message);

  EncodableMap response;
  response[EncodableValue("protocolVersion")] = EncodableValue(1);
  response[EncodableValue("requestId")] = EncodableValue(request_id);
  response[EncodableValue("status")] = EncodableValue("error");
  response[EncodableValue("result")] = EncodableValue();
  response[EncodableValue("error")] = EncodableValue(std::move(error));
  return EncodableValue(std::move(response));
}

bool ShowSystemNotification(HWND hwnd,
                            const std::wstring& title,
                            const std::wstring& body,
                            bool silent,
                            UINT* notification_id) {
  static std::atomic<UINT> next_id{0xA110};
  const UINT id = next_id.fetch_add(1, std::memory_order_relaxed);

  NOTIFYICONDATAW icon{};
  icon.cbSize = sizeof(icon);
  icon.hWnd = hwnd;
  icon.uID = id;
  icon.uFlags = NIF_ICON | NIF_TIP;
  icon.hIcon = static_cast<HICON>(LoadImageW(
      GetModuleHandleW(nullptr), MAKEINTRESOURCEW(IDI_APP_ICON), IMAGE_ICON, 0, 0,
      LR_DEFAULTSIZE | LR_SHARED));
  StringCchCopyW(icon.szTip, ARRAYSIZE(icon.szTip), L"Amitia");

  if (!Shell_NotifyIconW(NIM_ADD, &icon)) {
    return false;
  }

  icon.uFlags = NIF_INFO;
  StringCchCopyW(icon.szInfoTitle, ARRAYSIZE(icon.szInfoTitle), title.c_str());
  StringCchCopyW(icon.szInfo, ARRAYSIZE(icon.szInfo), body.c_str());
  icon.dwInfoFlags = silent ? NIIF_INFO | NIIF_NOSOUND : NIIF_INFO;
  const bool posted = Shell_NotifyIconW(NIM_MODIFY, &icon) != FALSE;
  if (!posted) {
    Shell_NotifyIconW(NIM_DELETE, &icon);
    return false;
  }

  *notification_id = id;
  std::thread([hwnd, id]() {
    std::this_thread::sleep_for(std::chrono::seconds(12));
    NOTIFYICONDATAW cleanup{};
    cleanup.cbSize = sizeof(cleanup);
    cleanup.hWnd = hwnd;
    cleanup.uID = id;
    Shell_NotifyIconW(NIM_DELETE, &cleanup);
  }).detach();
  return true;
}

std::string MciErrorMessage(MCIERROR error) {
  wchar_t buffer[256] = {};
  if (mciGetErrorStringW(error, buffer, ARRAYSIZE(buffer))) {
    return Utf8FromUtf16(buffer);
  }
  return "Windows multimedia command failed";
}

std::string IanaTimezoneFromWindowsKey(const std::string& windows_key) {
  if (windows_key == "UTC") return "UTC";
  if (windows_key == "China Standard Time") return "Asia/Shanghai";
  if (windows_key == "Tokyo Standard Time") return "Asia/Tokyo";
  if (windows_key == "Pacific Standard Time") return "America/Los_Angeles";
  if (windows_key == "Eastern Standard Time") return "America/New_York";
  if (windows_key == "GMT Standard Time") return "Europe/London";
  if (windows_key == "W. Europe Standard Time") return "Europe/Berlin";
  if (windows_key == "AUS Eastern Standard Time") return "Australia/Sydney";
  return std::string();
}

EncodableValue HandleNativeExecute(HWND hwnd, const EncodableValue* arguments) {
  const auto* request = arguments == nullptr
                            ? nullptr
                            : std::get_if<EncodableMap>(arguments);
  if (request == nullptr) {
    return ErrorResponse("", "INVALID_ARGUMENT",
                         "execute requires a request payload");
  }

  const std::string request_id = GetString(*request, "requestId");
  const std::string platform = GetString(*request, "platform");
  const std::string operation = GetString(*request, "operation");
  if (request_id.empty()) {
    return ErrorResponse(request_id, "INVALID_ARGUMENT",
                         "requestId must not be empty");
  }
  if (!platform.empty() && platform != "windows") {
    return ErrorResponse(request_id, "INVALID_PLATFORM",
                         "unsupported platform: " + platform);
  }
  if (operation.empty()) {
    return ErrorResponse(request_id, "INVALID_ARGUMENT",
                         "operation must not be empty");
  }

  static const EncodableMap empty_payload;
  const EncodableMap* payload = GetMap(*request, "payload");
  if (payload == nullptr) {
    payload = &empty_payload;
  }

  if (operation == "device.timezone.get") {
    DYNAMIC_TIME_ZONE_INFORMATION timezone_info{};
    const DWORD timezone_state = GetDynamicTimeZoneInformation(&timezone_info);
    if (timezone_state == TIME_ZONE_ID_INVALID) {
      return ErrorResponse(request_id, "TIMEZONE_UNAVAILABLE",
                           "Windows could not resolve the current timezone");
    }
    const wchar_t* key_name = timezone_info.TimeZoneKeyName[0] != L'\0'
                                  ? timezone_info.TimeZoneKeyName
                                  : timezone_info.StandardName;
    const std::string windows_timezone = Utf8FromUtf16(key_name);
    if (windows_timezone.empty()) {
      return ErrorResponse(request_id, "TIMEZONE_UNAVAILABLE",
                           "Windows timezone identifier is empty");
    }
    const std::string iana_timezone = IanaTimezoneFromWindowsKey(windows_timezone);
    EncodableMap result;
    result[EncodableValue("timezone")] = EncodableValue(windows_timezone);
    result[EncodableValue("windowsTimezone")] = EncodableValue(windows_timezone);
    if (!iana_timezone.empty()) {
      result[EncodableValue("ianaTimezone")] = EncodableValue(iana_timezone);
    }
    result[EncodableValue("source")] = EncodableValue("windows.system");
    return SuccessResponse(request_id, std::move(result));
  }

  if (operation == "notification.post") {
    const std::string title_utf8 = GetString(*payload, "title");
    const std::string body_utf8 = GetString(*payload, "body");
    if (title_utf8.empty() && body_utf8.empty()) {
      return ErrorResponse(request_id, "NOTIFICATION_POST_FAILED",
                           "both title and body are empty");
    }
    const std::wstring title = WideFromUtf8(title_utf8.empty() ? "Amitia" : title_utf8);
    const std::wstring body = WideFromUtf8(body_utf8);
    if (title.empty() && !title_utf8.empty()) {
      return ErrorResponse(request_id, "INVALID_ARGUMENT",
                           "notification title is not valid UTF-8");
    }
    if (body.empty() && !body_utf8.empty()) {
      return ErrorResponse(request_id, "INVALID_ARGUMENT",
                           "notification body is not valid UTF-8");
    }
    UINT notification_id = 0;
    if (!ShowSystemNotification(hwnd, title, body,
                                GetBool(*payload, "silent", false),
                                &notification_id)) {
      return ErrorResponse(request_id, "NOTIFICATION_POST_FAILED",
                           "Windows notification area rejected the notification");
    }
    EncodableMap result;
    result[EncodableValue("posted")] = EncodableValue(true);
    result[EncodableValue("notificationRef")] =
        EncodableValue(std::to_string(notification_id));
    return SuccessResponse(request_id, std::move(result));
  }

  if (operation == "media.audio.play_file") {
    const std::string path_utf8 = GetString(*payload, "path");
    if (path_utf8.empty()) {
      return ErrorResponse(request_id, "INVALID_ARGUMENT",
                           "audio file path is required");
    }
    const std::wstring path = WideFromUtf8(path_utf8);
    if (path.empty()) {
      return ErrorResponse(request_id, "INVALID_ARGUMENT",
                           "audio file path is not valid UTF-8");
    }
    const DWORD attributes = GetFileAttributesW(path.c_str());
    if (attributes == INVALID_FILE_ATTRIBUTES ||
        (attributes & FILE_ATTRIBUTE_DIRECTORY) != 0) {
      return ErrorResponse(request_id, "AUDIO_FILE_UNAVAILABLE",
                           "audio file is unavailable: " + path_utf8);
    }

    mciSendStringW(L"close amitia_voice_preview", nullptr, 0, nullptr);
    const std::wstring open_command =
        L"open \"" + path + L"\" alias amitia_voice_preview";
    MCIERROR error =
        mciSendStringW(open_command.c_str(), nullptr, 0, nullptr);
    if (error != 0) {
      return ErrorResponse(request_id, "AUDIO_PLAYBACK_FAILED",
                           MciErrorMessage(error));
    }
    error = mciSendStringW(L"play amitia_voice_preview", nullptr, 0, nullptr);
    if (error != 0) {
      mciSendStringW(L"close amitia_voice_preview", nullptr, 0, nullptr);
      return ErrorResponse(request_id, "AUDIO_PLAYBACK_FAILED",
                           MciErrorMessage(error));
    }

    wchar_t length_buffer[64] = {};
    int duration_ms = 0;
    if (mciSendStringW(L"status amitia_voice_preview length", length_buffer,
                       ARRAYSIZE(length_buffer), nullptr) == 0) {
      duration_ms = _wtoi(length_buffer);
    }
    EncodableMap result;
    result[EncodableValue("playing")] = EncodableValue(true);
    result[EncodableValue("path")] = EncodableValue(path_utf8);
    result[EncodableValue("durationMs")] = EncodableValue(duration_ms);
    return SuccessResponse(request_id, std::move(result));
  }

  if (operation == "media.audio.stop") {
    mciSendStringW(L"stop amitia_voice_preview", nullptr, 0, nullptr);
    mciSendStringW(L"close amitia_voice_preview", nullptr, 0, nullptr);
    EncodableMap result;
    result[EncodableValue("playing")] = EncodableValue(false);
    return SuccessResponse(request_id, std::move(result));
  }

  return ErrorResponse(request_id, "OPERATION_NOT_SUPPORTED",
                       "operation not supported: " + operation);
}

}  // namespace

FlutterWindow::FlutterWindow(const flutter::DartProject& project)
    : project_(project) {}

FlutterWindow::~FlutterWindow() {}

bool FlutterWindow::OnCreate() {
  if (!Win32Window::OnCreate()) {
    return false;
  }

  RECT frame = GetClientArea();

  // The size here must match the window dimensions to avoid unnecessary surface
  // creation / destruction in the startup path.
  flutter_controller_ = std::make_unique<flutter::FlutterViewController>(
      frame.right - frame.left, frame.bottom - frame.top, project_);
  // Ensure that basic setup of the controller was successful.
  if (!flutter_controller_->engine() || !flutter_controller_->view()) {
    return false;
  }
  RegisterPlugins(flutter_controller_->engine());

  native_bridge_channel_ =
      std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
          flutter_controller_->engine()->messenger(),
          "com.amitia.windows_native/bridge",
          &flutter::StandardMethodCodec::GetInstance());
  native_bridge_channel_->SetMethodCallHandler(
      [this](const auto& call, auto result) {
        if (call.method_name() == "nativeBridge.health") {
          EncodableMap health;
          health[EncodableValue("ready")] = EncodableValue(true);
          health[EncodableValue("foreground")] =
              EncodableValue(GetForegroundWindow() == GetHandle());
          health[EncodableValue("platform")] = EncodableValue("windows");
          health[EncodableValue("health")] = EncodableValue("ready");
          result->Success(EncodableValue(std::move(health)));
          return;
        }
        if (call.method_name() == "nativeBridge.execute") {
          result->Success(HandleNativeExecute(GetHandle(), call.arguments()));
          return;
        }
        result->NotImplemented();
      });

  SetChildContent(flutter_controller_->view()->GetNativeWindow());

  flutter_controller_->engine()->SetNextFrameCallback([&]() {
    this->Show();
  });

  // Flutter can complete the first frame before the "show window" callback is
  // registered. The following call ensures a frame is pending to ensure the
  // window is shown. It is a no-op if the first frame hasn't completed yet.
  flutter_controller_->ForceRedraw();

  return true;
}

void FlutterWindow::OnDestroy() {
  mciSendStringW(L"close amitia_voice_preview", nullptr, 0, nullptr);
  native_bridge_channel_.reset();
  if (flutter_controller_) {
    flutter_controller_ = nullptr;
  }

  Win32Window::OnDestroy();
}

LRESULT
FlutterWindow::MessageHandler(HWND hwnd, UINT const message,
                              WPARAM const wparam,
                              LPARAM const lparam) noexcept {
  // Give Flutter, including plugins, an opportunity to handle window messages.
  if (flutter_controller_) {
    std::optional<LRESULT> result =
        flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam,
                                                      lparam);
    if (result) {
      return *result;
    }
  }

  switch (message) {
    case WM_FONTCHANGE:
      flutter_controller_->engine()->ReloadSystemFonts();
      break;
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}
