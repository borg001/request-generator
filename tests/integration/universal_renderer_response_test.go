package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	module "github.com/darkrain/request-generator"
	"github.com/darkrain/request-generator/actions"
	dbpkg "github.com/darkrain/request-generator/db"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	"github.com/go-jet/jet/v2/postgres"
	pg "github.com/go-jet/jet/v2/postgres"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRendererDB struct{}

func (fakeRendererDB) List(
	_ *log.Entry,
	_ pg.Table,
	_ pg.Column,
	_ []fields.ModuleField,
	_ map[string]fields.ModuleFilterField,
	_ int64,
	_ int64,
	_ []pg.Column,
	_ string,
	_ map[string]string,
	_ pg.BoolExpression,
	_ []actions.ModuleActionJoin,
	_ *actions.SortOption,
	_ *dbpkg.TranslationContext,
) ([]interface{}, int64, error) {
	return []interface{}{}, 0, nil
}

func (fakeRendererDB) View(_ *log.Entry, _ pg.Table, _ pg.Column, _ []fields.ModuleField, _ pg.BoolExpression, _ []actions.ModuleActionJoin, _ *dbpkg.TranslationContext) (interface{}, error) {
	return map[string]interface{}{"id": 1, "status": "active", "avatar": "ipfs://avatar"}, nil
}

func (fakeRendererDB) Add(_ *log.Entry, _ pg.Table, _ pg.Column, _ []fields.ModuleField, _ map[string]interface{}, _ *dbpkg.TranslationContext) (interface{}, error) {
	return nil, nil
}

func (fakeRendererDB) Update(_ *log.Entry, _ pg.Table, _ pg.Column, _ []fields.ModuleField, _ map[string]interface{}, _ pg.BoolExpression, _ *dbpkg.TranslationContext) (interface{}, error) {
	return nil, nil
}

func (fakeRendererDB) Delete(_ *log.Entry, _ pg.Table, _ pg.BoolExpression, _ *dbpkg.TranslationContext) error {
	return nil
}

func (fakeRendererDB) RawRequest(_ *log.Entry, _ string, _ ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (fakeRendererDB) RawDB() *sql.DB {
	return nil
}

func setupUniversalRendererRouter(t *testing.T, pageHooks ...module.PageRenderFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	status := postgres.StringColumn("status")
	avatar := postgres.StringColumn("avatar")
	table := postgres.NewTable("public", "renderer_items", "", id, status, avatar)

	testModule := &module.BaseModule{
		Name:       "renderer-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
			{Column: status, Title: "Status", Type: fields.ModuleFieldTypeString, FormType: fields.ModuleFieldFormTypeSelect},
			{
				Column:   avatar,
				Title:    "Avatar",
				Type:     fields.ModuleFieldTypeString,
				FormType: fields.ModuleFieldFormTypeText,
				Presentation: &renderer.FieldPresentation{
					Renderer: renderer.RendererAvatar,
					Variant:  "avatar",
					Size:     renderer.MediaSizeThumb,
					Ratio:    renderer.MediaRatioSquare,
				},
				Media: &renderer.FieldMediaConfig{
					Item: &renderer.MediaGalleryItem{
						Kind:       renderer.MediaKindPhoto,
						Visibility: renderer.MediaVisibilityPublic,
						Usage:      renderer.MediaUsageAvatar,
					},
					Upload: &renderer.MediaUploadConfig{
						Title:        "Upload avatar",
						LoadingTitle: "Uploading avatar",
						Accept:       "image/jpeg,image/png,image/webp",
					},
					Labels: &renderer.MediaGalleryLabels{
						Empty:  "No avatar",
						Remove: "Remove avatar",
					},
					Actions: &renderer.MediaGalleryActions{
						Upload: &renderer.Action{ID: "upload", Label: "Upload", Type: renderer.ActionEmit, ActionPresentation: renderer.ActionPresentation{Icon: "upload"}},
						Crop:   &renderer.Action{ID: "crop", Label: "Crop", Type: renderer.ActionEmit, ActionPresentation: renderer.ActionPresentation{Icon: "crop"}},
						Remove: &renderer.Action{ID: "remove", Label: "Remove", Type: renderer.ActionAPI, API: &renderer.APIAction{Method: "POST", Endpoint: "/profiles/avatar/remove"}},
					},
					Cropper: &renderer.MediaCropperConfig{
						Title:        "Adjust image",
						Subtitle:     "Move and scale the image",
						Hint:         "Drag or pinch to zoom",
						ChooseLabel:  "Choose image",
						CancelLabel:  "Cancel",
						ConfirmLabel: "Use image",
						CloseLabel:   "Close",
						Accept:       "image/jpeg,image/png,image/webp",
						Viewport: renderer.MediaCropperViewportConfig{
							Shape:       renderer.MediaCropperViewportCircle,
							AspectRatio: 1,
						},
						Output: renderer.MediaCropperOutputConfig{
							Width:    512,
							Height:   512,
							MIMEType: renderer.MediaCropperOutputMIMETypeJPEG,
							Quality:  0.92,
						},
					},
				},
			},
		},
		Render: renderer.Universal{
			List: &renderer.ListPage{
				ID: "renderer-items",
				Layout: &renderer.Layout{
					Type:  renderer.LayoutOneColumn,
					Gap:   renderer.SpacingMD,
					Align: renderer.AlignStretch,
				},
			},
			Form: &renderer.FormPage{
				ID:     "renderer-items-form",
				Layout: renderer.LayoutTwoColumn,
			},
			Record: &renderer.RecordPage{
				Layout: &renderer.Layout{Type: renderer.LayoutThreeColumn},
			},
		},
		Actions: []actions.ModuleAction{
			actions.ListModuleAction{
				Columns:    []pg.Column{id, status},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "List",
			},
			actions.AddModuleAction{
				Columns:    []pg.Column{status, avatar},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "Add",
			},
			actions.ViewModuleAction{
				Columns:    []pg.Column{id, status, avatar},
				By:         []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "View",
			},
		},
	}

	if len(pageHooks) > 0 {
		testModule.PageRenderFunc = pageHooks[0]
	}
	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()
	return engine
}

func TestUniversalRendererMetadata_ListResponse(t *testing.T) {
	engine := setupUniversalRendererRouter(t)

	w := executeRequest(engine, http.MethodGet, "/admin/renderer-items", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Renderer *renderer.Identity `json:"renderer"`
		ListPage *renderer.ListPage `json:"list_page"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	require.NotNil(t, response.Renderer)
	assert.Equal(t, renderer.Name, response.Renderer.Name)
	assert.Equal(t, renderer.Version, response.Renderer.Version)
	require.NotNil(t, response.ListPage)
	assert.Equal(t, "renderer-items", response.ListPage.ID)
	assert.Equal(t, renderer.LayoutOneColumn, response.ListPage.Layout.Type)
	assert.Equal(t, renderer.SpacingMD, response.ListPage.Layout.Gap)
}

func TestUniversalRendererMetadata_DefrecResponse(t *testing.T) {
	engine := setupUniversalRendererRouter(t)

	w := executeRequest(engine, http.MethodGet, "/admin/renderer-items/defrec/", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Renderer *renderer.Identity `json:"renderer"`
		FormPage *renderer.FormPage `json:"form_page"`
		Fields   map[string]struct {
			Presentation *renderer.FieldPresentation `json:"presentation"`
			Media        *renderer.FieldMediaConfig  `json:"media"`
		} `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	require.NotNil(t, response.Renderer)
	assert.Equal(t, renderer.Name, response.Renderer.Name)
	assert.Equal(t, renderer.Version, response.Renderer.Version)
	require.NotNil(t, response.FormPage)
	assert.Equal(t, "renderer-items-form", response.FormPage.ID)
	assert.Equal(t, renderer.LayoutTwoColumn, response.FormPage.Layout)
	avatarField := response.Fields["avatar"]
	require.NotNil(t, avatarField.Presentation)
	assert.Equal(t, renderer.RendererAvatar, avatarField.Presentation.Renderer)
	assert.Equal(t, "avatar", avatarField.Presentation.Variant)
	require.NotNil(t, avatarField.Media)
	require.NotNil(t, avatarField.Media.Item)
	assert.Equal(t, renderer.MediaUsageAvatar, avatarField.Media.Item.Usage)
	require.NotNil(t, avatarField.Media.Upload)
	assert.Equal(t, "Upload avatar", avatarField.Media.Upload.Title)
	require.NotNil(t, avatarField.Media.Actions)
	require.NotNil(t, avatarField.Media.Actions.Crop)
	assert.Equal(t, "Crop", avatarField.Media.Actions.Crop.Label)
	require.NotNil(t, avatarField.Media.Actions.Remove)
	assert.Equal(t, "/profiles/avatar/remove", avatarField.Media.Actions.Remove.API.Endpoint)
	require.NotNil(t, avatarField.Media.Cropper)
	assert.Equal(t, renderer.MediaCropperViewportCircle, avatarField.Media.Cropper.Viewport.Shape)
	assert.Equal(t, 512, avatarField.Media.Cropper.Output.Width)
	assert.Equal(t, "Use image", avatarField.Media.Cropper.ConfirmLabel)
}

func TestUniversalRendererMetadata_FormSectionMediaGallery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	table := postgres.NewTable("public", "media_renderer_items", "", id)

	testModule := &module.BaseModule{
		Name:       "media-renderer-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
		},
		Render: renderer.Universal{
			Form: &renderer.FormPage{
				ID:     "media-renderer-items-form",
				Layout: renderer.LayoutTwoColumn,
				Sections: []renderer.FormSection{
					{
						ID:       "media",
						Renderer: renderer.RendererMediaGallery,
						MediaUpload: &renderer.MediaUploadConfig{
							Title:        "Upload media",
							Subtitle:     "Images and videos",
							LoadingTitle: "Uploading",
							Accept:       "image/jpeg,video/mp4",
							Multiple:     true,
						},
						MediaItems: []renderer.MediaGalleryItem{
							{
								ID:         "media-link-1",
								MediaID:    10,
								LinkID:     1,
								Kind:       renderer.MediaKindVideo,
								Src:        "ipfs://video",
								Poster:     "ipfs://poster",
								Visibility: renderer.MediaVisibilityPublic,
								HideFace:   true,
								Usage:      renderer.MediaUsageGallery,
								SortOrder:  1,
							},
						},
						MediaLabels: &renderer.MediaGalleryLabels{
							Public:     "Public",
							Private:    "Private",
							Empty:      "No media yet",
							CoverBadge: "Cover",
						},
						MediaActions: &renderer.MediaGalleryActions{
							Upload: &renderer.Action{
								ID:   "upload",
								Type: renderer.ActionAPI,
								API:  &renderer.APIAction{Method: "PUT", Endpoint: "/media_assets"},
							},
							Update: &renderer.Action{
								ID:   "update",
								Type: renderer.ActionAPI,
								API:  &renderer.APIAction{Method: "POST", Endpoint: "/media_links/id/{link_id}"},
							},
							Remove: &renderer.Action{
								ID:   "remove",
								Type: renderer.ActionAPI,
								API:  &renderer.APIAction{Method: "DELETE", Endpoint: "/media_links/:id"},
							},
						},
					},
				},
			},
		},
		Defrec: actions.DefrecModuleAction{
			Permission: []actions.Role{actions.RoleAll},
			Auth:       true,
			Label:      "Defrec",
		},
		Actions: []actions.ModuleAction{
			actions.AddModuleAction{
				Columns:    []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "Add",
			},
		},
	}

	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()

	w := executeRequest(engine, http.MethodGet, "/admin/media-renderer-items/defrec/", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		FormPage *renderer.FormPage `json:"form_page"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotNil(t, response.FormPage)
	require.Len(t, response.FormPage.Sections, 1)

	section := response.FormPage.Sections[0]
	require.NotNil(t, section.MediaUpload)
	assert.Equal(t, "Upload media", section.MediaUpload.Title)
	require.Len(t, section.MediaItems, 1)
	assert.Equal(t, renderer.MediaKindVideo, section.MediaItems[0].Kind)
	assert.Equal(t, renderer.MediaVisibilityPublic, section.MediaItems[0].Visibility)
	assert.Equal(t, renderer.MediaUsageGallery, section.MediaItems[0].Usage)
	require.NotNil(t, section.MediaLabels)
	assert.Equal(t, "No media yet", section.MediaLabels.Empty)
	require.NotNil(t, section.MediaActions)
	require.NotNil(t, section.MediaActions.Upload)
	assert.Equal(t, "/media_assets", section.MediaActions.Upload.API.Endpoint)
}

func TestUniversalRendererMetadata_FormSectionCollectionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	table := postgres.NewTable("public", "collection_renderer_items", "", id)
	profileID := postgres.IntegerColumn("profile_id")
	profilesTable := postgres.NewTable("public", "profiles", "", id)
	tagsTable := postgres.NewTable("public", "tags", "", id, profileID)
	servicesTable := postgres.NewTable("public", "services", "", id, profileID)
	active := true

	testModule := &module.BaseModule{
		Name:       "collection-renderer-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
		},
		Render: renderer.Universal{
			Form: &renderer.FormPage{
				ID:     "collection-renderer-items-form",
				Layout: renderer.LayoutTwoColumn,
				Sections: []renderer.FormSection{
					{
						ID:       "simple_collection",
						Renderer: renderer.RendererCollectionManager,
						Collection: &renderer.CollectionConfig{
							Module:   "tags",
							Relation: "owner",
							Item:     &renderer.CollectionItem{LabelField: "title"},
							Buckets: []renderer.CollectionBucket{
								{ID: "all", Title: "All", BlockID: "collection.default"},
							},
						},
					},
					{
						ID:       "editable_collection",
						Renderer: renderer.RendererCollectionManager,
						Collection: &renderer.CollectionConfig{
							Module:     "services",
							EditFields: []string{"price", "note", "available"},
							Item:       &renderer.CollectionItem{LabelField: "title", MetaFields: []string{"price", "note"}},
							Buckets: []renderer.CollectionBucket{
								{
									ID:         "included",
									Title:      "Included",
									BlockID:    "collection.included",
									EditFields: []string{"note"},
									Predicate: &renderer.CollectionPredicate{
										Field:    "price",
										Operator: renderer.CollectionPredicateEquals,
										Value:    &renderer.TypedValue{Type: renderer.TypedValueNumber, Number: 0},
									},
									Defaults: []renderer.CollectionFieldDefaultValue{
										{Field: "price", Value: renderer.TypedValue{Type: renderer.TypedValueNumber, Number: 0}},
										{Field: "available", Value: renderer.TypedValue{Type: renderer.TypedValueBool, Bool: &active}},
									},
								},
								{
									ID:      "paid",
									Title:   "Paid",
									BlockID: "collection.paid",
									Block: &renderer.Block{
										Type:    renderer.BlockType("GlassesPanel"),
										Variant: renderer.BlockVariant("compact-info"),
									},
									Predicate: &renderer.CollectionPredicate{
										Field:    "price",
										Operator: renderer.CollectionPredicateGreaterThan,
										Value:    &renderer.TypedValue{Type: renderer.TypedValueNumber, Number: 0},
									},
								},
							},
						},
					},
				},
			},
		},
		Defrec: actions.DefrecModuleAction{
			Permission: []actions.Role{actions.RoleAll},
			Auth:       true,
			Label:      "Defrec",
		},
		Actions: []actions.ModuleAction{
			actions.AddModuleAction{
				Columns:    []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "Add",
			},
		},
	}

	engine := gin.New()
	group := engine.Group("")
	_ = profilesTable
	tagsModule := &module.BaseModule{
		Name:       "tags",
		Path:       "/admin",
		Table:      tagsTable,
		PrimaryKey: id,
		Relations: []module.ModuleRelation{{
			Name:         "owner",
			TargetModule: "collection-renderer-items",
			SourceField:  profileID,
			TargetField:  id,
			ScopeCheck:   func(_ *gin.Context, _ module.RelationScope) error { return nil },
		}},
	}
	servicesModule := &module.BaseModule{
		Name:       "services",
		Path:       "/admin",
		Table:      servicesTable,
		PrimaryKey: id,
	}
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule, tagsModule, servicesModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()

	w := executeRequest(engine, http.MethodGet, "/admin/collection-renderer-items/defrec/", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		FormPage *renderer.FormPage `json:"form_page"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotNil(t, response.FormPage)
	require.Len(t, response.FormPage.Sections, 2)

	simple := response.FormPage.Sections[0].Collection
	require.NotNil(t, simple)
	assert.Equal(t, "tags", simple.Module)
	assert.Empty(t, simple.EditFields)
	assert.Equal(t, "owner", simple.Relation)
	assert.Equal(t, "title", simple.Item.LabelField)
	require.Len(t, simple.Buckets, 1)
	assert.Equal(t, "collection.default", simple.Buckets[0].BlockID)

	editable := response.FormPage.Sections[1].Collection
	require.NotNil(t, editable)
	assert.Equal(t, "services", editable.Module)
	assert.Equal(t, []string{"price", "note", "available"}, editable.EditFields)
	assert.Equal(t, []string{"price", "note"}, editable.Item.MetaFields)
	require.Len(t, editable.Buckets, 2)
	included := editable.Buckets[0]
	assert.Equal(t, "collection.included", included.BlockID)
	assert.Equal(t, []string{"note"}, included.EditFields)
	require.NotNil(t, included.Predicate)
	assert.Equal(t, "price", included.Predicate.Field)
	assert.Equal(t, renderer.CollectionPredicateEquals, included.Predicate.Operator)
	require.NotNil(t, included.Predicate.Value)
	assert.Equal(t, float64(0), included.Predicate.Value.Number)
	require.Len(t, included.Defaults, 2)
	assert.Equal(t, "price", included.Defaults[0].Field)
	assert.Equal(t, renderer.TypedValueNumber, included.Defaults[0].Value.Type)
	assert.Equal(t, "available", included.Defaults[1].Field)
	require.NotNil(t, included.Defaults[1].Value.Bool)
	assert.True(t, *included.Defaults[1].Value.Bool)
	paid := editable.Buckets[1]
	require.NotNil(t, paid.Block)
	assert.Equal(t, renderer.BlockType("GlassesPanel"), paid.Block.Type)
	assert.Equal(t, renderer.BlockVariant("compact-info"), paid.Block.Variant)
}

func TestUniversalRendererMetadata_CollectionValidation(t *testing.T) {
	err := (renderer.Universal{
		Form: &renderer.FormPage{
			Sections: []renderer.FormSection{
				{ID: "collection", Renderer: renderer.RendererCollectionManager, Collection: &renderer.CollectionConfig{}},
			},
		},
	}).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `collection section "collection" must define module`)

	err = (renderer.Universal{
		Form: &renderer.FormPage{
			Sections: []renderer.FormSection{
				{
					ID:       "collection",
					Renderer: renderer.RendererCollectionManager,
					Collection: &renderer.CollectionConfig{
						Module: "items",
						Buckets: []renderer.CollectionBucket{
							{ID: "bad", Predicate: &renderer.CollectionPredicate{Operator: renderer.CollectionPredicateEquals}},
						},
					},
				},
			},
		},
	}).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `collection bucket "bad" predicate must define field`)

	err = (renderer.Universal{
		Form: &renderer.FormPage{
			Sections: []renderer.FormSection{
				{
					ID:       "collection",
					Renderer: renderer.RendererCollectionManager,
					Collection: &renderer.CollectionConfig{
						Module: "items",
						Buckets: []renderer.CollectionBucket{
							{
								ID: "bad",
								Predicate: &renderer.CollectionPredicate{
									Field:    "status",
									Operator: renderer.CollectionPredicateIn,
									Value:    &renderer.TypedValue{Type: renderer.TypedValueString, String: "active"},
									Values:   []renderer.TypedValue{{Type: renderer.TypedValueString, String: "draft"}},
								},
							},
						},
					},
				},
			},
		},
	}).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `collection bucket "bad" predicate must not define both value and values`)
}

func TestUniversalRendererMetadata_CollectionRelationValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	profileID := postgres.IntegerColumn("profile_id")
	pageTable := postgres.NewTable("public", "collection_page", "", id)
	itemsTable := postgres.NewTable("public", "items", "", id, profileID)

	newGenerator := func(collectionModule *module.BaseModule, collectionName string, relation string) *module.Generator {
		pageModule := &module.BaseModule{
			Name:       "collection-page",
			Path:       "/admin",
			Table:      pageTable,
			PrimaryKey: id,
			Render: renderer.Universal{
				Form: &renderer.FormPage{
					Sections: []renderer.FormSection{
						{
							ID:       "items",
							Renderer: renderer.RendererCollectionManager,
							Collection: &renderer.CollectionConfig{
								Module:   collectionName,
								Relation: relation,
							},
						},
					},
				},
			},
		}
		modules := []*module.BaseModule{pageModule}
		if collectionModule != nil {
			modules = append(modules, collectionModule)
		}
		engine := gin.New()
		group := engine.Group("")
		return module.NewGenerator(
			func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
			*group,
			modules,
			func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
				return func(c *gin.Context) { c.Next() }
			},
			createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
		)
	}

	assert.PanicsWithValue(t,
		`invalid collection config in module collection-page: collection section "items" references unknown module "items"`,
		func() { newGenerator(nil, "items", "owner").Run() },
	)

	itemsWithoutRelation := &module.BaseModule{Name: "items", Path: "/admin", Table: itemsTable, PrimaryKey: id}
	assert.PanicsWithValue(t,
		`invalid collection config in module collection-page: collection module "items" must declare relation "owner"`,
		func() { newGenerator(itemsWithoutRelation, "items", "owner").Run() },
	)

	itemsWithRelationToWrongTarget := &module.BaseModule{
		Name:       "items",
		Path:       "/admin",
		Table:      itemsTable,
		PrimaryKey: id,
		Relations: []module.ModuleRelation{{
			Name:         "owner",
			TargetModule: "profiles",
			SourceField:  profileID,
			TargetField:  id,
			ScopeCheck:   func(_ *gin.Context, _ module.RelationScope) error { return nil },
		}},
	}
	assert.PanicsWithValue(t,
		`invalid collection config in module collection-page: collection module "items" relation "owner" must target module "collection-page"`,
		func() { newGenerator(itemsWithRelationToWrongTarget, "items", "owner").Run() },
	)

	itemsWithoutScopeCheck := &module.BaseModule{
		Name:       "items",
		Path:       "/admin",
		Table:      itemsTable,
		PrimaryKey: id,
		Relations:  []module.ModuleRelation{{Name: "owner", TargetModule: "collection-page", SourceField: profileID, TargetField: id}},
	}
	assert.PanicsWithValue(t,
		`invalid collection config in module collection-page: collection module "items" relation "owner" must declare ScopeCheck`,
		func() { newGenerator(itemsWithoutScopeCheck, "items", "owner").Run() },
	)

	itemsWithRelation := &module.BaseModule{
		Name:       "items",
		Path:       "/admin",
		Table:      itemsTable,
		PrimaryKey: id,
		Relations: []module.ModuleRelation{{
			Name:         "owner",
			TargetModule: "collection-page",
			SourceField:  profileID,
			TargetField:  id,
			ScopeCheck:   func(_ *gin.Context, _ module.RelationScope) error { return nil },
		}},
	}
	assert.NotPanics(t, func() { newGenerator(itemsWithRelation, "items", "owner").Run() })

	unscopedItems := &module.BaseModule{Name: "items", Path: "/admin", Table: itemsTable, PrimaryKey: id}
	assert.NotPanics(t, func() { newGenerator(unscopedItems, "items", "").Run() })
}

func TestUniversalRendererMetadata_ViewResponse(t *testing.T) {
	engine := setupUniversalRendererRouter(t)

	w := executeRequest(engine, http.MethodGet, "/admin/renderer-items/view/id/1", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Renderer   *renderer.Identity   `json:"renderer"`
		RecordPage *renderer.RecordPage `json:"record_page"`
		Item       map[string]struct {
			Value        interface{}                 `json:"value"`
			Presentation *renderer.FieldPresentation `json:"presentation"`
			Media        *renderer.FieldMediaConfig  `json:"media"`
		} `json:"item"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	require.NotNil(t, response.Renderer)
	assert.Equal(t, renderer.Name, response.Renderer.Name)
	assert.Equal(t, renderer.Version, response.Renderer.Version)
	require.NotNil(t, response.RecordPage)
	assert.Equal(t, renderer.LayoutThreeColumn, response.RecordPage.Layout.Type)
	avatarField := response.Item["avatar"]
	assert.Equal(t, "ipfs://avatar", avatarField.Value)
	require.NotNil(t, avatarField.Presentation)
	assert.Equal(t, renderer.RendererAvatar, avatarField.Presentation.Renderer)
	require.NotNil(t, avatarField.Media)
	require.NotNil(t, avatarField.Media.Item)
	assert.Equal(t, renderer.MediaUsageAvatar, avatarField.Media.Item.Usage)
	assert.Equal(t, "ipfs://avatar", avatarField.Media.Item.Src)
	require.NotNil(t, avatarField.Media.Actions)
	assert.Equal(t, "Remove", avatarField.Media.Actions.Remove.Label)
	require.NotNil(t, avatarField.Media.Cropper)
	assert.Equal(t, 1.0, avatarField.Media.Cropper.Viewport.AspectRatio)
	assert.Equal(t, renderer.MediaCropperOutputMIMETypeJPEG, avatarField.Media.Cropper.Output.MIMEType)
}

func TestUniversalRendererMetadata_ViewFormPageResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	status := postgres.StringColumn("status")
	table := postgres.NewTable("public", "renderer_settings", "", id, status)

	testModule := &module.BaseModule{
		Name:       "renderer-settings",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
			{
				Column:       status,
				Title:        "Status",
				Type:         fields.ModuleFieldTypeString,
				FormType:     fields.ModuleFieldFormTypeText,
				RoleFormType: map[string]fields.ModuleFieldFormType{"admin": fields.ModuleFieldFormTypeSelect},
			},
		},
		Render: renderer.Universal{
			Form: &renderer.FormPage{
				ID:     "renderer-settings-form",
				Layout: renderer.LayoutTwoColumn,
			},
			Record: &renderer.RecordPage{
				ID:     "renderer-settings-record",
				Layout: &renderer.Layout{Type: renderer.LayoutThreeColumn},
			},
		},
		Actions: []actions.ModuleAction{
			actions.ViewModuleAction{
				Columns:    []pg.Column{id, status},
				By:         []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "Settings",
				PageTypeFunc: func(c *gin.Context) renderer.PageType {
					if c.Query("settings") == "1" {
						return renderer.PageTypeForm
					}
					return renderer.PageTypeRecord
				},
			},
		},
		Navigation: []module.NavigationEntry{
			{
				ActionName: "view",
				Title:      "Settings",
				Group:      "settings",
				Show:       true,
				Path:       "/settings",
				Target: module.NavigationTarget{
					Type:     "page",
					PageType: renderer.PageTypeForm,
					Params: map[string]interface{}{
						"bykey": "current",
						"value": "current",
					},
				},
			},
		},
	}

	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()

	w := executeRequest(engine, http.MethodGet, "/api/config", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var config module.ConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &config))
	require.Len(t, config.Navigation, 1)
	assert.Equal(t, renderer.PageTypeForm, config.Navigation[0].Target.PageType)
	require.NotNil(t, config.Navigation[0].Target.Query)
	assert.Equal(t, "/api/admin/renderer-settings/view/:bykey/:value", config.Navigation[0].Target.Query.Url)

	w = executeRequest(engine, http.MethodGet, "/admin/renderer-settings/view/id/1?settings=1", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Renderer   *renderer.Identity   `json:"renderer"`
		FormPage   *renderer.FormPage   `json:"form_page"`
		RecordPage *renderer.RecordPage `json:"record_page"`
		Item       map[string]struct {
			FormType string `json:"form_type"`
		} `json:"item"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotNil(t, response.Renderer)
	require.NotNil(t, response.FormPage)
	assert.Equal(t, "renderer-settings-form", response.FormPage.ID)
	assert.Nil(t, response.RecordPage)
	assert.Equal(t, string(fields.ModuleFieldFormTypeSelect), response.Item["status"].FormType)

	w = executeRequest(engine, http.MethodGet, "/admin/renderer-settings/view/id/1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	response = struct {
		Renderer   *renderer.Identity   `json:"renderer"`
		FormPage   *renderer.FormPage   `json:"form_page"`
		RecordPage *renderer.RecordPage `json:"record_page"`
		Item       map[string]struct {
			FormType string `json:"form_type"`
		} `json:"item"`
	}{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotNil(t, response.RecordPage)
	assert.Nil(t, response.FormPage)
}

func TestCheckRequest_ValidatesSubmittedFieldsOutsideActionColumns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	status := postgres.StringColumn("status")
	table := postgres.NewTable("public", "guarded_items", "", id, status)

	testModule := &module.BaseModule{
		Name:       "guarded-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
			{
				Column:   status,
				Title:    "Status",
				Type:     fields.ModuleFieldTypeString,
				FormType: fields.ModuleFieldFormTypeSelect,
				Check: []fields.CheckRules{
					fields.DataRule(func(_ *gin.Context, _ *sql.DB, data map[string]interface{}, _ string) error {
						if _, exists := data["status"]; exists {
							return fmt.Errorf("status is read-only")
						}
						return nil
					}, []fields.Scenario{fields.ScenarioUpdate}),
				},
			},
		},
		Actions: []actions.ModuleAction{
			actions.UpdateModuleAction{
				Columns:    []pg.Column{id},
				By:         []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "Update",
			},
		},
	}

	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()

	w := executeJSONRequest(engine, http.MethodPost, "/admin/guarded-items/id/1", map[string]interface{}{
		"status": "verified",
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	var response struct {
		Message string            `json:"message"`
		Errors  map[string]string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "status is read-only", response.Message)
	assert.Equal(t, map[string]string{"status": "status is read-only"}, response.Errors)
}

func TestUniversalRendererMetadata_ListAndResourceGridAreMutuallyExclusive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	table := postgres.NewTable("public", "invalid_renderer_items", "", id)

	testModule := &module.BaseModule{
		Name:       "invalid-renderer-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
		},
		Render: renderer.Universal{
			List:         &renderer.ListPage{ID: "invalid-renderer-items"},
			ResourceGrid: &renderer.ResourceGridPage{Endpoint: "/invalid-renderer-items"},
		},
		Actions: []actions.ModuleAction{
			actions.ListModuleAction{
				Columns:    []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "List",
			},
		},
	}

	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)

	assert.PanicsWithValue(
		t,
		"invalid renderer config in module invalid-renderer-items: renderer.Universal: List and ResourceGrid are mutually exclusive for one list route",
		func() { generator.Run() },
	)
}

func TestUniversalRendererMetadata_RenderFuncBuildsTypedRuntimeMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	table := postgres.NewTable("public", "dynamic_renderer_items", "", id)

	testModule := &module.BaseModule{
		Name:       "dynamic-renderer-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
		},
		RenderFunc: func(c *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			base.List = &renderer.ListPage{
				ID: "dynamic-" + c.Query("scope"),
				Layout: &renderer.Layout{
					Type: renderer.LayoutOneColumn,
					Gap:  renderer.SpacingLG,
				},
			}
			return base, nil
		},
		Navigation: []module.NavigationEntry{
			{ActionName: "list", Title: "Dynamic", Group: "Admin", Order: 1, Show: true, Path: "/admin/dynamic-renderer-items"},
		},
		Actions: []actions.ModuleAction{
			actions.ListModuleAction{
				Columns:    []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "List",
			},
		},
	}

	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()

	w := executeRequest(engine, http.MethodGet, "/admin/dynamic-renderer-items?scope=models", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var listResponse struct {
		Renderer *renderer.Identity `json:"renderer"`
		ListPage *renderer.ListPage `json:"list_page"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResponse))
	require.NotNil(t, listResponse.Renderer)
	require.NotNil(t, listResponse.ListPage)
	assert.Equal(t, "dynamic-models", listResponse.ListPage.ID)
	assert.Equal(t, renderer.SpacingLG, listResponse.ListPage.Layout.Gap)

	w = executeRequest(engine, http.MethodGet, "/api/config?scope=config", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var configResponse module.ConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &configResponse))
	var entry *module.ConfigNavigationEntry
	for i := range configResponse.Navigation {
		if configResponse.Navigation[i].Path == "/admin/dynamic-renderer-items" {
			entry = &configResponse.Navigation[i]
			break
		}
	}
	require.NotNil(t, entry)
	require.NotNil(t, entry.Target.Renderer)
	assert.Equal(t, renderer.PageTypeList, entry.Target.PageType)
}

func TestUniversalRendererMetadata_RenderFuncResultIsValidated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	table := postgres.NewTable("public", "invalid_dynamic_renderer_items", "", id)

	testModule := &module.BaseModule{
		Name:       "invalid-dynamic-renderer-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
		},
		RenderFunc: func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			base.List = &renderer.ListPage{ID: "invalid-dynamic-renderer-items"}
			base.ResourceGrid = &renderer.ResourceGridPage{Endpoint: "/invalid-dynamic-renderer-items"}
			return base, nil
		},
		Actions: []actions.ModuleAction{
			actions.ListModuleAction{
				Columns:    []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "List",
			},
		},
	}

	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()

	w := executeRequest(engine, http.MethodGet, "/admin/invalid-dynamic-renderer-items", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "renderer.Universal: List and ResourceGrid are mutually exclusive for one list route")
}

func TestUniversalRendererMetadata_RenderFuncDoesNotMutateBaseRender(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := postgres.IntegerColumn("id")
	table := postgres.NewTable("public", "isolated_renderer_items", "", id)

	baseRender := renderer.Universal{
		Form: &renderer.FormPage{
			ID: "isolated-renderer-items-form",
			Context: map[string]interface{}{
				"base":   true,
				"labels": []string{"base"},
				"items": []interface{}{
					map[string]interface{}{"id": "base"},
				},
				"nested": map[string]interface{}{
					"initial": true,
				},
			},
			Sections: []renderer.FormSection{
				{ID: "base", Title: "Base"},
			},
		},
	}

	testModule := &module.BaseModule{
		Name:       "isolated-renderer-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
		},
		Render: baseRender,
		RenderFunc: func(c *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			if c.Query("role") == "agency" {
				base.Form.Context["can_manage_models"] = true
				base.Form.Context["nested"].(map[string]interface{})["agency"] = true
				base.Form.Context["items"].([]interface{})[0].(map[string]interface{})["agency"] = true
				labels := base.Form.Context["labels"].([]string)
				labels[0] = "agency"
				base.Form.Context["labels"] = labels
				base.Form.Sections = append(base.Form.Sections, renderer.FormSection{ID: "agency", Title: "Agency"})
			}
			return base, nil
		},
		Actions: []actions.ModuleAction{
			actions.AddModuleAction{
				Columns:    []pg.Column{id},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
				Label:      "Add",
			},
		},
	}

	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
		*group,
		[]*module.BaseModule{testModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()

	w := executeRequest(engine, http.MethodGet, "/admin/isolated-renderer-items/defrec/?role=agency", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var agencyResponse struct {
		FormPage *renderer.FormPage `json:"form_page"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &agencyResponse))
	require.NotNil(t, agencyResponse.FormPage)
	assert.Equal(t, true, agencyResponse.FormPage.Context["can_manage_models"])
	assert.Equal(t, true, agencyResponse.FormPage.Context["nested"].(map[string]interface{})["agency"])
	assert.Equal(t, true, agencyResponse.FormPage.Context["items"].([]interface{})[0].(map[string]interface{})["agency"])
	assert.Equal(t, "agency", agencyResponse.FormPage.Context["labels"].([]interface{})[0])
	assert.Len(t, agencyResponse.FormPage.Sections, 2)

	w = executeRequest(engine, http.MethodGet, "/admin/isolated-renderer-items/defrec/?role=client", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var clientResponse struct {
		FormPage *renderer.FormPage `json:"form_page"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &clientResponse))
	require.NotNil(t, clientResponse.FormPage)
	assert.Equal(t, true, clientResponse.FormPage.Context["base"])
	assert.Equal(t, true, clientResponse.FormPage.Context["nested"].(map[string]interface{})["initial"])
	assert.NotContains(t, clientResponse.FormPage.Context["nested"].(map[string]interface{}), "agency")
	assert.NotContains(t, clientResponse.FormPage.Context["items"].([]interface{})[0].(map[string]interface{}), "agency")
	assert.Equal(t, "base", clientResponse.FormPage.Context["labels"].([]interface{})[0])
	assert.NotContains(t, clientResponse.FormPage.Context, "can_manage_models")
	assert.Len(t, clientResponse.FormPage.Sections, 1)

	assert.NotContains(t, testModule.Render.Form.Context, "can_manage_models")
	assert.NotContains(t, testModule.Render.Form.Context["nested"].(map[string]interface{}), "agency")
	assert.NotContains(t, testModule.Render.Form.Context["items"].([]interface{})[0].(map[string]interface{}), "agency")
	assert.Equal(t, "base", testModule.Render.Form.Context["labels"].([]string)[0])
	assert.Len(t, testModule.Render.Form.Sections, 1)
}

func TestPageRenderEndpointsPreserveCompleteResponses(t *testing.T) {
	legacy := setupUniversalRendererRouter(t)
	selected := []renderer.PageType{}
	pages := setupUniversalRendererRouter(t, func(c *gin.Context, draft *renderer.Draft) error {
		selected = append(selected, draft.PageType())
		return nil
	})
	for _, path := range []string{"/admin/renderer-items", "/admin/renderer-items/defrec/", "/admin/renderer-items/view/id/1"} {
		before := executeRequest(legacy, http.MethodGet, path, nil)
		after := executeRequest(pages, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, before.Code, before.Body.String())
		require.Equal(t, before.Code, after.Code, after.Body.String())
		require.JSONEq(t, before.Body.String(), after.Body.String(), path)
	}
	require.Equal(t, []renderer.PageType{renderer.PageTypeList, renderer.PageTypeForm, renderer.PageTypeRecord}, selected)
}
