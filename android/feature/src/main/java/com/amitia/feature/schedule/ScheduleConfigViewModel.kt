package com.amitia.feature.schedule

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.UiError
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class ScheduleEditViewModel @Inject constructor() : ViewModel() {

    private val _form = MutableStateFlow(ScheduleFormState())
    val form: StateFlow<ScheduleFormState> = _form.asStateFlow()

    private val _saving = MutableStateFlow(false)
    val saving: StateFlow<Boolean> = _saving.asStateFlow()

    fun update(transform: (ScheduleFormState) -> ScheduleFormState) {
        _form.value = transform(_form.value)
    }

    fun save(onDone: () -> Unit) {
        viewModelScope.launch {
            _saving.value = true
            kotlinx.coroutines.delay(500)
            _saving.value = false
            onDone()
        }
    }
}

data class LifeTemplateData(
    val templates: List<LifeTemplate> = emptyList()
)

@HiltViewModel
class LifeTemplateViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<LifeTemplateData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<LifeTemplateData>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                kotlinx.coroutines.delay(300)
                LifeTemplateData(ScheduleMockData.lifeTemplates)
            }.onSuccess { data ->
                _state.value = if (data.templates.isEmpty()) ScreenState.Empty()
                else ScreenState.Content(data)
            }.onFailure { _state.value = ScreenState.Error(UiError(title = "加载模板失败", message = it.message ?: "")) }
        }
    }

    fun toggleTemplate(id: String) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(
            current.copy(templates = current.templates.map {
                if (it.id == id) it.copy(enabled = !it.enabled) else it
            })
        )
    }

    fun duplicateTemplate(id: String) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        val source = current.templates.firstOrNull { it.id == id } ?: return
        val copy = source.copy(
            id = "${id}_copy_${System.currentTimeMillis()}",
            name = "${source.name} 副本",
            enabled = false
        )
        val index = current.templates.indexOf(source)
        val newTemplates = current.templates.toMutableList().apply { add(index + 1, copy) }
        _state.value = ScreenState.Content(current.copy(templates = newTemplates))
    }
}

data class ProactiveWindowData(
    val window: ProactiveMessageWindow = ScheduleMockData.proactiveWindow
)

@HiltViewModel
class ProactiveWindowViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<ProactiveWindowData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<ProactiveWindowData>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                kotlinx.coroutines.delay(300)
                ProactiveWindowData(ScheduleMockData.proactiveWindow)
            }.onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "加载配置失败", message = it.message ?: "")) }
        }
    }

    fun update(transform: (ProactiveMessageWindow) -> ProactiveMessageWindow) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(current.copy(window = transform(current.window)))
    }
}

data class StateRuleData(
    val rules: List<StateRule> = emptyList()
)

@HiltViewModel
class StateRuleViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<StateRuleData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<StateRuleData>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                kotlinx.coroutines.delay(300)
                StateRuleData(ScheduleMockData.stateRules)
            }.onSuccess { data ->
                _state.value = if (data.rules.isEmpty()) ScreenState.Empty()
                else ScreenState.Content(data)
            }.onFailure { _state.value = ScreenState.Error(UiError(title = "加载规则失败", message = it.message ?: "")) }
        }
    }

    fun toggleRule(id: String) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(
            current.copy(rules = current.rules.map {
                if (it.id == id) it.copy(enabled = !it.enabled) else it
            })
        )
    }
}

data class QuietHoursData(
    val items: List<QuietHoursConfig> = emptyList()
)

@HiltViewModel
class QuietHoursViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<QuietHoursData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<QuietHoursData>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                kotlinx.coroutines.delay(300)
                QuietHoursData(ScheduleMockData.quietHours)
            }.onSuccess { data ->
                _state.value = if (data.items.isEmpty()) ScreenState.Empty()
                else ScreenState.Content(data)
            }.onFailure { _state.value = ScreenState.Error(UiError(title = "加载安静时段失败", message = it.message ?: "")) }
        }
    }

    fun toggle(id: String) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(
            current.copy(items = current.items.map {
                if (it.id == id) it.copy(enabled = !it.enabled) else it
            })
        )
    }

    fun update(id: String, transform: (QuietHoursConfig) -> QuietHoursConfig) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(
            current.copy(items = current.items.map {
                if (it.id == id) transform(it) else it
            })
        )
    }
}
