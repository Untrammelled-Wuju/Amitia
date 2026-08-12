package card

import "errors"

var (
	ErrUnsupportedFormat  = errors.New("character_card_unsupported_format")
	ErrInvalidCard        = errors.New("character_card_invalid")
	ErrCardTooLarge       = errors.New("character_card_too_large")
	ErrJSONInvalid        = errors.New("character_card_json_invalid")
	ErrPNGMetadataMissing = errors.New("character_card_png_metadata_missing")
	ErrPNGMetadataInvalid = errors.New("character_card_png_metadata_invalid")
	ErrCHARXInvalid       = errors.New("character_card_charx_invalid")
	ErrAssetInvalid       = errors.New("character_card_asset_invalid")
	ErrAssetTooLarge      = errors.New("character_card_asset_too_large")
	ErrFutureVersion      = errors.New("character_card_future_version")
	ErrImportFailed       = errors.New("character_card_import_failed")
	ErrExportFailed       = errors.New("character_card_export_failed")
	ErrPromptTooLarge     = errors.New("character_card_prompt_too_large")
	ErrLorebookTooLarge   = errors.New("character_card_lorebook_too_large")
)
