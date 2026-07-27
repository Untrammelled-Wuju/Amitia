package com.amitia.feature.character

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.model.CharacterDto
import com.amitia.core.repository.CharacterRepository
import com.amitia.feature.character.model.ArchivedCharacter
import com.amitia.feature.character.model.AppearanceAsset
import com.amitia.feature.character.model.CapabilityItem
import com.amitia.feature.character.model.ChannelBinding
import com.amitia.feature.character.model.CharacterDataStats
import com.amitia.feature.character.model.CharacterEmotionState
import com.amitia.feature.character.model.CharacterOverviewData
import com.amitia.feature.character.model.CharacterRelationship
import com.amitia.feature.character.model.CharacterSampleData
import com.amitia.feature.character.model.CharacterThemeColor
import com.amitia.feature.character.model.LifeStatusItem
import com.amitia.feature.character.model.MemoryConfig
import com.amitia.feature.character.model.ModelBindingConfig
import com.amitia.feature.character.model.PetActionItem
import com.amitia.feature.character.model.PetAssetSet
import com.amitia.feature.character.model.PermissionItem
import com.amitia.feature.character.model.PersonalityData
import com.amitia.feature.character.model.PersonalityPreset
import com.amitia.feature.character.model.ProactiveMessageRule
import com.amitia.feature.character.model.RelationshipEvent
import com.amitia.feature.character.model.VoiceConfig
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class CharacterDetailViewModel @Inject constructor(
    private val characterRepository: CharacterRepository
) : ViewModel() {

    private val _overviewState = MutableStateFlow<ScreenState<CharacterOverviewData>>(ScreenState.Loading)
    val overviewState: StateFlow<ScreenState<CharacterOverviewData>> = _overviewState.asStateFlow()

    private val _personalityState = MutableStateFlow<ScreenState<List<com.amitia.feature.character.model.PersonalityGroup>>>(ScreenState.Loading)
    val personalityState: StateFlow<ScreenState<List<com.amitia.feature.character.model.PersonalityGroup>>> = _personalityState.asStateFlow()

    private val _relationshipState = MutableStateFlow<ScreenState<Pair<List<CharacterRelationship>, List<RelationshipEvent>>>>(ScreenState.Loading)
    val relationshipState: StateFlow<ScreenState<Pair<List<CharacterRelationship>, List<RelationshipEvent>>>> = _relationshipState.asStateFlow()

    private val _emotionState = MutableStateFlow<ScreenState<CharacterEmotionState>>(ScreenState.Loading)
    val emotionState: StateFlow<ScreenState<CharacterEmotionState>> = _emotionState.asStateFlow()

    private val _lifeStatusState = MutableStateFlow<ScreenState<List<LifeStatusItem>>>(ScreenState.Loading)
    val lifeStatusState: StateFlow<ScreenState<List<LifeStatusItem>>> = _lifeStatusState.asStateFlow()

    private val _proactiveState = MutableStateFlow<ScreenState<ProactiveMessageRule>>(ScreenState.Loading)
    val proactiveState: StateFlow<ScreenState<ProactiveMessageRule>> = _proactiveState.asStateFlow()

    private val _voiceState = MutableStateFlow<ScreenState<VoiceConfig>>(ScreenState.Loading)
    val voiceState: StateFlow<ScreenState<VoiceConfig>> = _voiceState.asStateFlow()

    private val _modelBindingState = MutableStateFlow<ScreenState<ModelBindingConfig>>(ScreenState.Loading)
    val modelBindingState: StateFlow<ScreenState<ModelBindingConfig>> = _modelBindingState.asStateFlow()

    private val _memoryState = MutableStateFlow<ScreenState<MemoryConfig>>(ScreenState.Loading)
    val memoryState: StateFlow<ScreenState<MemoryConfig>> = _memoryState.asStateFlow()

    private val _channelState = MutableStateFlow<ScreenState<List<ChannelBinding>>>(ScreenState.Loading)
    val channelState: StateFlow<ScreenState<List<ChannelBinding>>> = _channelState.asStateFlow()

    private val _capabilityState = MutableStateFlow<ScreenState<List<CapabilityItem>>>(ScreenState.Loading)
    val capabilityState: StateFlow<ScreenState<List<CapabilityItem>>> = _capabilityState.asStateFlow()

    private val _permissionState = MutableStateFlow<ScreenState<List<PermissionItem>>>(ScreenState.Loading)
    val permissionState: StateFlow<ScreenState<List<PermissionItem>>> = _permissionState.asStateFlow()

    private val _dataStatsState = MutableStateFlow<ScreenState<CharacterDataStats>>(ScreenState.Loading)
    val dataStatsState: StateFlow<ScreenState<CharacterDataStats>> = _dataStatsState.asStateFlow()

    private val _archiveState = MutableStateFlow<ScreenState<List<ArchivedCharacter>>>(ScreenState.Loading)
    val archiveState: StateFlow<ScreenState<List<ArchivedCharacter>>> = _archiveState.asStateFlow()

    private val _appearanceState = MutableStateFlow<ScreenState<List<AppearanceAsset>>>(ScreenState.Loading)
    val appearanceState: StateFlow<ScreenState<List<AppearanceAsset>>> = _appearanceState.asStateFlow()

    private val _petAssetState = MutableStateFlow<ScreenState<PetAssetSet>>(ScreenState.Loading)
    val petAssetState: StateFlow<ScreenState<PetAssetSet>> = _petAssetState.asStateFlow()

    private val _petActionState = MutableStateFlow<ScreenState<List<PetActionItem>>>(ScreenState.Loading)
    val petActionState: StateFlow<ScreenState<List<PetActionItem>>> = _petActionState.asStateFlow()

    private var currentCharacter: CharacterDto? = null

    fun loadAll(characterId: String) {
        loadOverview(characterId)
        loadPersonality()
        loadRelationships()
        loadEmotion()
        loadLifeStatus()
        loadProactive()
        loadVoice()
        loadModelBinding()
        loadMemory()
        loadChannels()
        loadCapabilities()
        loadPermissions()
        loadDataStats()
        loadArchive()
        loadAppearance()
        loadPetAssets()
        loadPetActions()
    }

    fun loadOverview(characterId: String) {
        _overviewState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching { characterRepository.get(characterId) }
                .onSuccess { character ->
                    currentCharacter = character
                    _overviewState.value = ScreenState.Content(CharacterSampleData.sampleOverview(character))
                }
                .onFailure {
                    _overviewState.value = ScreenState.Error(
                        com.amitia.core.designsystem.UiError(
                            title = "加载失败",
                            message = it.message ?: "无法加载角色概览",
                            type = com.amitia.core.designsystem.ErrorType.OperationFailed
                        )
                    )
                }
        }
    }

    fun loadPersonality() {
        _personalityState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _personalityState.value = ScreenState.Content(PersonalityData.groups)
        }
    }

    fun loadRelationships() {
        _relationshipState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _relationshipState.value = ScreenState.Content(
                CharacterSampleData.sampleRelationships() to CharacterSampleData.sampleRelationshipEvents()
            )
        }
    }

    fun loadEmotion() {
        _emotionState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _emotionState.value = ScreenState.Content(CharacterSampleData.sampleEmotionState())
        }
    }

    fun loadLifeStatus() {
        _lifeStatusState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _lifeStatusState.value = ScreenState.Content(CharacterSampleData.sampleLifeStatuses())
        }
    }

    fun loadProactive() {
        _proactiveState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _proactiveState.value = ScreenState.Content(CharacterSampleData.sampleProactiveRule())
        }
    }

    fun loadVoice() {
        _voiceState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _voiceState.value = ScreenState.Content(CharacterSampleData.sampleVoiceConfig())
        }
    }

    fun loadModelBinding() {
        _modelBindingState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _modelBindingState.value = ScreenState.Content(CharacterSampleData.sampleModelBinding())
        }
    }

    fun loadMemory() {
        _memoryState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _memoryState.value = ScreenState.Content(CharacterSampleData.sampleMemoryConfig())
        }
    }

    fun loadChannels() {
        _channelState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _channelState.value = ScreenState.Content(CharacterSampleData.sampleChannelBindings())
        }
    }

    fun loadCapabilities() {
        _capabilityState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _capabilityState.value = ScreenState.Content(CharacterSampleData.sampleCapabilities())
        }
    }

    fun loadPermissions() {
        _permissionState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _permissionState.value = ScreenState.Content(CharacterSampleData.samplePermissions())
        }
    }

    fun loadDataStats() {
        _dataStatsState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _dataStatsState.value = ScreenState.Content(CharacterSampleData.sampleDataStats())
        }
    }

    fun loadArchive() {
        _archiveState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _archiveState.value = ScreenState.Content(CharacterSampleData.sampleArchivedCharacters())
        }
    }

    fun loadAppearance() {
        _appearanceState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _appearanceState.value = ScreenState.Content(CharacterSampleData.sampleAppearanceAssets())
        }
    }

    fun loadPetAssets() {
        _petAssetState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _petAssetState.value = ScreenState.Content(CharacterSampleData.samplePetAssetSet())
        }
    }

    fun loadPetActions() {
        _petActionState.value = ScreenState.Loading
        viewModelScope.launch {
            delay(200)
            _petActionState.value = ScreenState.Content(CharacterSampleData.samplePetActions())
        }
    }

    fun retryAll(characterId: String) {
        loadAll(characterId)
    }
}
