package renderer

const (
	Name    = "UniversalRenderer"
	Version = "2.6.0"
)

type Identity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func UniversalIdentity() Identity {
	return Identity{Name: Name, Version: Version}
}

type PageType string

const (
	PageTypeList         PageType = "list"
	PageTypeForm         PageType = "form"
	PageTypeRecord       PageType = "record"
	PageTypeResourceGrid PageType = "resource_grid"
)

type SpacingToken string

const (
	SpacingNone SpacingToken = "none"
	SpacingXS   SpacingToken = "xs"
	SpacingSM   SpacingToken = "sm"
	SpacingMD   SpacingToken = "md"
	SpacingLG   SpacingToken = "lg"
	SpacingXL   SpacingToken = "xl"
)

type RadiusToken string

const (
	RadiusNone RadiusToken = "none"
	RadiusSM   RadiusToken = "sm"
	RadiusMD   RadiusToken = "md"
	RadiusLG   RadiusToken = "lg"
	RadiusXL   RadiusToken = "xl"
	RadiusFull RadiusToken = "full"
)

type InsetToken string

const (
	InsetNone       InsetToken = "none"
	InsetInlineMD   InsetToken = "inline-md"
	InsetMediaFrame InsetToken = "media-frame"
)

type ComponentRadiusToken string

const (
	ComponentRadiusNone     ComponentRadiusToken = "none"
	ComponentRadiusMediaTop ComponentRadiusToken = "media-top"
)

type SizeToken string

const (
	SizeXS SizeToken = "xs"
	SizeSM SizeToken = "sm"
	SizeMD SizeToken = "md"
	SizeLG SizeToken = "lg"
	SizeXL SizeToken = "xl"
)

type WeightToken string

const (
	WeightRegular  WeightToken = "regular"
	WeightMedium   WeightToken = "medium"
	WeightSemibold WeightToken = "semibold"
	WeightBold     WeightToken = "bold"
)

type AlignToken string

const (
	AlignStart   AlignToken = "start"
	AlignCenter  AlignToken = "center"
	AlignEnd     AlignToken = "end"
	AlignStretch AlignToken = "stretch"
)

type ToneToken string

const (
	ToneDefault ToneToken = "default"
	ToneMuted   ToneToken = "muted"
	ToneSoft    ToneToken = "soft"
	ToneAccent  ToneToken = "accent"
	ToneSuccess ToneToken = "success"
	ToneWarning ToneToken = "warning"
	ToneDanger  ToneToken = "danger"
)

type LayoutType string

const (
	LayoutOneColumn   LayoutType = "one_column"
	LayoutTwoColumn   LayoutType = "two_column"
	LayoutThreeColumn LayoutType = "three_column"
)

type MaxWidth string

const (
	MaxWidthNone MaxWidth = "none"
	MaxWidthSM   MaxWidth = "sm"
	MaxWidthMD   MaxWidth = "md"
	MaxWidthLG   MaxWidth = "lg"
	MaxWidthXL   MaxWidth = "xl"
	MaxWidthFull MaxWidth = "full"
)

type GridMode string

const (
	GridModeTable GridMode = "table"
	GridModeCards GridMode = "cards"
	GridModeList  GridMode = "list"
)

func (mode GridMode) Valid() bool {
	switch mode {
	case "", GridModeTable, GridModeCards, GridModeList:
		return true
	default:
		return false
	}
}

type GridColumnCount uint8

const (
	GridColumnsOne GridColumnCount = iota + 1
	GridColumnsTwo
	GridColumnsThree
	GridColumnsFour
	GridColumnsFive
	GridColumnsSix
)

func (count GridColumnCount) Valid() bool {
	return count >= GridColumnsOne && count <= GridColumnsSix
}

type PaginationMode string

const (
	PaginationServer PaginationMode = "server"
	PaginationClient PaginationMode = "client"
)

type CardVariant string

const (
	CardVariantDefault  CardVariant = "default"
	CardVariantMedia    CardVariant = "media"
	CardVariantCompact  CardVariant = "compact"
	CardVariantActivity CardVariant = "activity"
)

type ListGroupByType string

const (
	ListGroupByDate ListGroupByType = "date"
)

type TextFormat string

const (
	TextFormatRelativeTime TextFormat = "relative_time"
)

type SurfaceVariant string

const (
	SurfaceDefault   SurfaceVariant = "default"
	SurfacePrimary   SurfaceVariant = "primary"
	SurfaceSecondary SurfaceVariant = "secondary"
)

type SurfaceEffect string

const (
	SurfaceEffectNone     SurfaceEffect = "none"
	SurfaceEffectFlat     SurfaceEffect = "flat"
	SurfaceEffectElevated SurfaceEffect = "elevated"
)

type MediaRatio string

const (
	MediaRatioSquare    MediaRatio = "square"
	MediaRatioPortrait  MediaRatio = "portrait"
	MediaRatioLandscape MediaRatio = "landscape"
	MediaRatioWide      MediaRatio = "wide"
)

type MediaSize string

const (
	MediaSizeThumb MediaSize = "thumb"
	MediaSizeCard  MediaSize = "card"
	MediaSizeHero  MediaSize = "hero"
)

type BlockType string

const (
	BlockNone  BlockType = "none"
	BlockPanel BlockType = "panel"
	BlockCard  BlockType = "card"
)

type BlockVariant string

const (
	BlockVariantDefault BlockVariant = "default"
	BlockVariantCompact BlockVariant = "compact"
)

type TitleDecorToken string

const (
	TitleDecorNone    TitleDecorToken = "none"
	TitleDecorSection TitleDecorToken = "section"
)

type RendererKey string

const (
	RendererUniversalDisplay    RendererKey = "universal.display"
	RendererUniversalSection    RendererKey = "universal.section"
	RendererUniversalFilters    RendererKey = "universal.filters"
	RendererUniversalPagination RendererKey = "universal.pagination"
	RendererMediaGallery        RendererKey = "media.gallery"
	RendererCollectionManager   RendererKey = "collection.manager"
	RendererFieldMatrix         RendererKey = "field.matrix"
	RendererAvatar              RendererKey = "avatar"
	RendererBadge               RendererKey = "badge"
	RendererChipSelect          RendererKey = "chip_select"
	RendererPrimaryRadio        RendererKey = "primary_radio"
	RendererRecordSelect        RendererKey = "record_select"
	RendererDateRange           RendererKey = "date_range"
)

type RecordSectionRenderer = RendererKey

const (
	RecordRendererDisplay RecordSectionRenderer = RendererUniversalDisplay
)

type LayoutSlotToken string

const (
	LayoutSlotLeft   LayoutSlotToken = "left"
	LayoutSlotCenter LayoutSlotToken = "center"
	LayoutSlotRight  LayoutSlotToken = "right"
)

type ActionType string

const (
	ActionRoute    ActionType = "route"
	ActionAPI      ActionType = "api"
	ActionModal    ActionType = "modal"
	ActionEmit     ActionType = "emit"
	ActionExternal ActionType = "external"
)

type ActionVariant string

const (
	ActionVariantDefault   ActionVariant = "default"
	ActionVariantPrimary   ActionVariant = "primary"
	ActionVariantSecondary ActionVariant = "secondary"
	ActionVariantSuccess   ActionVariant = "success"
	ActionVariantWarning   ActionVariant = "warning"
	ActionVariantDanger    ActionVariant = "danger"
)

type ActionAppearance string

const (
	ActionAppearanceSolid    ActionAppearance = "solid"
	ActionAppearanceGradient ActionAppearance = "gradient"
	ActionAppearanceOutline  ActionAppearance = "outline"
	ActionAppearanceGhost    ActionAppearance = "ghost"
	ActionAppearanceSoft     ActionAppearance = "soft"
	ActionAppearanceLink     ActionAppearance = "link"
)

// ActionPlacement selects an optional generic action surface. An empty value
// leaves placement to the renderer that owns the action.
type ActionPlacement string

const (
	ActionPlacementFull         ActionPlacement = "full"
	ActionPlacementFilterFooter ActionPlacement = "filter_footer"
	ActionPlacementBadge        ActionPlacement = "badge"
	ActionPlacementHead         ActionPlacement = "head"
	ActionPlacementMenu         ActionPlacement = "menu"
)

func (placement ActionPlacement) Valid() bool {
	switch placement {
	case "", ActionPlacementFull, ActionPlacementFilterFooter, ActionPlacementBadge, ActionPlacementHead, ActionPlacementMenu:
		return true
	default:
		return false
	}
}

type MediaKind string

const (
	MediaKindPhoto MediaKind = "photo"
	MediaKindVideo MediaKind = "video"
	MediaKindFile  MediaKind = "file"
)

type MediaVisibility string

const (
	MediaVisibilityPublic   MediaVisibility = "public"
	MediaVisibilityPrivate  MediaVisibility = "private"
	MediaVisibilityPaid     MediaVisibility = "paid"
	MediaVisibilityInternal MediaVisibility = "internal"
)

type MediaUsage string

const (
	MediaUsageGallery MediaUsage = "gallery"
	MediaUsageAvatar  MediaUsage = "avatar"
	MediaUsagePoster  MediaUsage = "poster"
)

type MediaCropperViewportShape string

const (
	MediaCropperViewportCircle    MediaCropperViewportShape = "circle"
	MediaCropperViewportRounded   MediaCropperViewportShape = "rounded"
	MediaCropperViewportRectangle MediaCropperViewportShape = "rectangle"
)

type MediaCropperOutputMIMEType string

const (
	MediaCropperOutputMIMETypeJPEG MediaCropperOutputMIMEType = "image/jpeg"
	MediaCropperOutputMIMETypePNG  MediaCropperOutputMIMEType = "image/png"
	MediaCropperOutputMIMETypeWebP MediaCropperOutputMIMEType = "image/webp"
)

type DisplayComponentType string

const (
	DisplayMediaGallery    DisplayComponentType = "media_gallery"
	DisplayActions         DisplayComponentType = "actions"
	DisplayIdentity        DisplayComponentType = "identity"
	DisplayDataList        DisplayComponentType = "data_list"
	DisplayBadgeGroupBlock DisplayComponentType = "badge_group_block"
	DisplayText            DisplayComponentType = "text"
	DisplayBadgeList       DisplayComponentType = "badge_list"
	DisplayAccordionGroups DisplayComponentType = "accordion_groups"
	DisplayStatusTimeline  DisplayComponentType = "status_timeline"
)

type ComponentAction string

const (
	ComponentActionGallerySet ComponentAction = "gallery:set"
)

type ComponentDisplayType string

const (
	ComponentDisplayKeyValueGrid ComponentDisplayType = "key_value_grid"
	ComponentDisplayTileGrid     ComponentDisplayType = "tile_grid"
)

type ComponentRatio string

const (
	ComponentRatioSquare   ComponentRatio = "square"
	ComponentRatioPortrait ComponentRatio = "portrait"
)

type MediaOverlayPosition string

const (
	MediaOverlayTopLeft     MediaOverlayPosition = "top-left"
	MediaOverlayTopRight    MediaOverlayPosition = "top-right"
	MediaOverlayBottomLeft  MediaOverlayPosition = "bottom-left"
	MediaOverlayBottomRight MediaOverlayPosition = "bottom-right"
)

type DirectionToken string

const (
	DirectionRow    DirectionToken = "row"
	DirectionColumn DirectionToken = "column"
)

type JustifyToken string

const (
	JustifyStart   JustifyToken = "start"
	JustifyCenter  JustifyToken = "center"
	JustifyEnd     JustifyToken = "end"
	JustifyStretch JustifyToken = "stretch"
	JustifyBetween JustifyToken = "between"
)

type SeparatorAppearance string

const (
	SeparatorSolid    SeparatorAppearance = "solid"
	SeparatorGradient SeparatorAppearance = "gradient"
)
