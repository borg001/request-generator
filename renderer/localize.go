package renderer

type TextResolver func(value string, key string) string

type textLocalizer struct {
	resolve TextResolver
}

func (localizer textLocalizer) localizeRendererText(value, key string) string {
	if key != "" {
		return localizer.resolve(value, key)
	}
	return localizer.resolve(value, "")
}

func (localizer textLocalizer) localizeTextFields(fields ...*string) {
	for _, field := range fields {
		*field = localizer.localizeRendererText(*field, "")
	}
}

func (localizer textLocalizer) localizeRendererAction(action *Action) {
	if action == nil {
		return
	}
	action.Label = localizer.localizeRendererText(action.Label, action.LabelKey)
	action.LabelKey = ""
	action.AriaLabel = localizer.localizeRendererText(action.AriaLabel, action.AriaLabelKey)
	action.AriaLabelKey = ""
	action.Title = localizer.localizeRendererText(action.Title, action.TitleKey)
	action.TitleKey = ""
	localizer.localizeTextFields(&action.SavingLabel, &action.SavedLabel)
	if action.Modal != nil {
		localizer.localizeTextFields(&action.Modal.Title)
	}
	if action.Confirm != nil {
		localizer.localizeTextFields(&action.Confirm.Title, &action.Confirm.Message, &action.Confirm.CancelLabel, &action.Confirm.ConfirmLabel)
	}
	if action.AfterSuccess != nil {
		localizer.localizeTextFields(&action.AfterSuccess.Toast)
	}
	if action.AfterError != nil {
		localizer.localizeTextFields(&action.AfterError.Toast)
	}
}

func Localize(render Universal, resolve TextResolver) Universal {
	if resolve == nil {
		return render.Clone()
	}
	localized := render.Clone()
	return (textLocalizer{resolve: resolve}).localizeRenderer(localized)
}

func LocalizeFieldMedia(value *FieldMediaConfig, resolve TextResolver) *FieldMediaConfig {
	localized := CloneFieldMediaConfig(value)
	if localized == nil || resolve == nil {
		return localized
	}
	localizer := textLocalizer{resolve: resolve}
	localized.Upload = localizer.localizeMediaUpload(localized.Upload)
	localized.Labels = localizer.localizeMediaLabels(localized.Labels)
	localizer.localizeMediaActions(localized.Actions)
	localizer.localizeMediaGalleryItem(localized.Item)
	localizer.localizeMediaCropper(localized.Cropper)
	return localized
}

func (localizer textLocalizer) localizeRenderer(render Universal) Universal {
	if render.List != nil {
		localizer.localizeListPage(render.List)
	}
	if render.Form != nil {
		localizer.localizeFormPage(render.Form)
	}
	if render.Record != nil {
		localizer.localizeRecordPage(render.Record)
	}
	if render.ResourceGrid != nil {
		localizer.localizeResourceGridPage(render.ResourceGrid)
	}
	return render
}

func (localizer textLocalizer) localizeListPage(page *ListPage) {
	localizer.localizeTextFields(&page.Title, &page.Subtitle)
	for i := range page.Actions {
		localizer.localizeRendererAction(&page.Actions[i])
	}
	if page.Filters != nil {
		localizer.localizeFilterPills(page.Filters.PillRows)
		localizer.localizeFilterPills(page.Filters.SecondaryPillRows)
		localizer.localizeFilterGroups(page.Filters.Groups)
		localizer.localizeFilterText(page.Filters.Text)
		localizer.localizeFilterRangePresets(page.Filters.RangePresets)
	}
	if page.Summary != nil {
		localizer.localizeTextFields(&page.Summary.Title, &page.Summary.TitleFallback)
		for i := range page.Summary.Items {
			page.Summary.Items[i].Label = localizer.localizeRendererText(page.Summary.Items[i].Label, page.Summary.Items[i].LabelKey)
			page.Summary.Items[i].LabelKey = ""
		}
		if page.Summary.Trend != nil {
			trend := page.Summary.Trend
			localizer.localizeTextFields(&trend.Title, &trend.Subtitle)
			trend.AriaLabel = localizer.localizeRendererText(trend.AriaLabel, trend.AriaLabelKey)
			trend.AriaLabelKey = ""
			trend.EmptyLabel = localizer.localizeRendererText(trend.EmptyLabel, trend.EmptyLabelKey)
			trend.EmptyLabelKey = ""
			trend.LoadingLabel = localizer.localizeRendererText(trend.LoadingLabel, trend.LoadingLabelKey)
			trend.LoadingLabelKey = ""
			for i := range trend.Series {
				trend.Series[i].Label = localizer.localizeRendererText(trend.Series[i].Label, trend.Series[i].LabelKey)
				trend.Series[i].LabelKey = ""
			}
			localizer.localizeDateRangeToolbar(trend.DateRange)
		}
	}
	if page.Filters != nil {
		localizer.localizeDateRangeToolbar(page.Filters.DateRange)
	}
	if page.GroupBy != nil {
		localizer.localizeTextFields(&page.GroupBy.TodayLabel, &page.GroupBy.YesterdayLabel, &page.GroupBy.ThisWeekLabel, &page.GroupBy.EarlierLabel)
	}
	if page.CardSchema != nil {
		localizer.localizeCardSchema(page.CardSchema)
	}
	if page.Selection != nil {
		localizer.localizeTextFields(&page.Selection.SelectedLabel)
		localizer.localizeRendererAction(page.Selection.Clear)
		localizer.localizeRendererAction(page.Selection.Proceed)
	}
}

func (localizer textLocalizer) localizeDateRangeToolbar(toolbar *DateRangeToolbar) {
	if toolbar == nil {
		return
	}
	localizer.localizeTextFields(&toolbar.Placeholder, &toolbar.ApplyLabel, &toolbar.CancelLabel, &toolbar.StartLabel, &toolbar.EndLabel, &toolbar.EmptyLabel, &toolbar.DialogLabel, &toolbar.PreviousLabel, &toolbar.NextLabel)
	for i := range toolbar.Presets {
		toolbar.Presets[i].Label = localizer.localizeRendererText(toolbar.Presets[i].Label, toolbar.Presets[i].LabelKey)
		toolbar.Presets[i].LabelKey = ""
	}
	for i := range toolbar.Months {
		toolbar.Months[i] = localizer.localizeRendererText(toolbar.Months[i], "")
	}
	for i := range toolbar.FormatMonths {
		toolbar.FormatMonths[i] = localizer.localizeRendererText(toolbar.FormatMonths[i], "")
	}
	for i := range toolbar.Weekdays {
		toolbar.Weekdays[i] = localizer.localizeRendererText(toolbar.Weekdays[i], "")
	}
}

func (localizer textLocalizer) localizeFilterGroups(groups []FilterGroup) {
	for i := range groups {
		localizer.localizeFilterGroup(&groups[i])
	}
}

func (localizer textLocalizer) localizeFilterGroup(group *FilterGroup) {
	group.Label = localizer.localizeRendererText(group.Label, group.LabelKey)
	group.LabelKey = ""
	for j := range group.Sections {
		group.Sections[j].Label = localizer.localizeRendererText(group.Sections[j].Label, group.Sections[j].LabelKey)
		group.Sections[j].LabelKey = ""
	}
	for j := range group.Items {
		if group.Items[j].Group != nil {
			localizer.localizeFilterGroup(group.Items[j].Group)
		}
	}
}

func (localizer textLocalizer) localizeFilterRangePresets(groups []FilterRangePresets) {
	for i := range groups {
		for j := range groups[i].Presets {
			groups[i].Presets[j].Label = localizer.localizeRendererText(groups[i].Presets[j].Label, "")
		}
	}
}

func (localizer textLocalizer) localizeFilterText(text *FilterText) {
	if text != nil {
		localizer.localizeTextFields(&text.SearchPlaceholder, &text.ResetLabel, &text.ResetAllLabel, &text.ApplyLabel, &text.LoadingLabel, &text.EmptyLabel, &text.EmptyDescription, &text.NoResultsLabel, &text.CancelLabel, &text.CloseLabel, &text.RangeMinLabel, &text.RangeMaxLabel)
	}
}

func (localizer textLocalizer) localizeFilterPills(rows [][]FilterPill) {
	for i := range rows {
		for j := range rows[i] {
			pill := &rows[i][j]
			pill.Label = localizer.localizeRendererText(pill.Label, pill.LabelKey)
			pill.LabelKey = ""
			pill.GroupLabel = localizer.localizeRendererText(pill.GroupLabel, pill.GroupLabelKey)
			pill.GroupLabelKey = ""
		}
	}
}

func (localizer textLocalizer) localizeCardSchema(schema *CardSchema) {
	for i := range schema.Badges {
		localizer.localizeBadge(&schema.Badges[i])
	}
	for i := range schema.Stats {
		localizer.localizeBadge(&schema.Stats[i])
	}
	for i := range schema.Actions {
		localizer.localizeRendererAction(&schema.Actions[i])
	}
}

func (localizer textLocalizer) localizeBadge(badge *Badge) {
	if badge == nil {
		return
	}
	badge.Label = localizer.localizeRendererText(badge.Label, badge.LabelKey)
	badge.LabelKey = ""
	for value, label := range badge.LabelMap {
		badge.LabelMap[value] = localizer.localizeRendererText(label, "")
	}
	if badge.Then != nil {
		badge.Then.Label = localizer.localizeRendererText(badge.Then.Label, badge.Then.LabelKey)
		badge.Then.LabelKey = ""
	}
	if badge.Else != nil {
		badge.Else.Label = localizer.localizeRendererText(badge.Else.Label, badge.Else.LabelKey)
		badge.Else.LabelKey = ""
	}
}

func (localizer textLocalizer) localizeFormPage(page *FormPage) {
	localizer.localizeTextFields(&page.Title, &page.Subtitle)
	if page.Workflow != nil {
		localizer.localizeTextFields(&page.Workflow.PreviousLabel, &page.Workflow.NextLabel)
		if page.Workflow.Summary != nil {
			localizer.localizeTextFields(&page.Workflow.Summary.Eyebrow, &page.Workflow.Summary.Title)
			localizer.localizeBadge(page.Workflow.Summary.Badge)
		}
	}
	for i := range page.Actions {
		localizer.localizeRendererAction(&page.Actions[i])
	}
	for i := range page.Sections {
		localizer.localizeFormSection(&page.Sections[i])
	}
}

func (localizer textLocalizer) localizeFormSection(section *FormSection) {
	localizer.localizeTextFields(&section.Title, &section.StepHint, &section.PanelTitle, &section.Subtitle, &section.LoadingLabel, &section.GroupTitle)
	localizer.localizePromptList(section.Prompts)
	localizer.localizeFieldMatrix(section.Matrix)
	if section.ListPage != nil {
		localizer.localizeListPage(section.ListPage)
	}
	localizer.localizeCollection(section.Collection)
	section.MediaUpload = localizer.localizeMediaUpload(section.MediaUpload)
	section.MediaLabels = localizer.localizeMediaLabels(section.MediaLabels)
	localizer.localizeMediaActions(section.MediaActions)
	localizer.localizeMediaGalleryItems(section.MediaItems)
	localizer.localizeDateRange(section.DateRange)
}

func (localizer textLocalizer) localizeDateRange(config *DateRangeConfig) {
	if config == nil {
		return
	}
	localizer.localizeTextFields(&config.Placeholder, &config.ApplyLabel, &config.CancelLabel, &config.StartLabel, &config.EndLabel, &config.EmptyLabel, &config.DialogLabel, &config.PreviousLabel, &config.NextLabel)
	for index := range config.Months {
		config.Months[index] = localizer.localizeRendererText(config.Months[index], "")
	}
	for index := range config.FormatMonths {
		config.FormatMonths[index] = localizer.localizeRendererText(config.FormatMonths[index], "")
	}
	for index := range config.Weekdays {
		config.Weekdays[index] = localizer.localizeRendererText(config.Weekdays[index], "")
	}
}

func (localizer textLocalizer) localizePromptList(list *PromptList) {
	if list == nil {
		return
	}
	for index := range list.Items {
		prompt := &list.Items[index]
		localizer.localizeTextFields(&prompt.Title, &prompt.Text)
		localizer.localizeRendererAction(prompt.Action)
	}
}

func (localizer textLocalizer) localizeFieldMatrix(matrix *FieldMatrix) {
	if matrix == nil || matrix.Table == nil {
		return
	}
	for i := range matrix.Table.Heads {
		matrix.Table.Heads[i] = localizer.localizeRendererText(matrix.Table.Heads[i], "")
	}
	for i := range matrix.Table.Rows {
		row := &matrix.Table.Rows[i]
		localizer.localizeTextFields(&row.Label, &row.Description)
		for j := range row.Cells {
			localizer.localizeTextFields(&row.Cells[j].Label, &row.Cells[j].Text)
		}
	}
}

func (localizer textLocalizer) localizeMediaUpload(upload *MediaUploadConfig) *MediaUploadConfig {
	if upload != nil {
		localizer.localizeTextFields(&upload.Title, &upload.Subtitle, &upload.LoadingTitle)
	}
	return upload
}

func (localizer textLocalizer) localizeMediaLabels(labels *MediaGalleryLabels) *MediaGalleryLabels {
	if labels != nil {
		localizer.localizeTextFields(&labels.Public, &labels.Private, &labels.Empty, &labels.CoverBadge, &labels.Remove, &labels.Reorder, &labels.FirstIsCover, &labels.PrivateHint, &labels.HideFace, &labels.HideFaceHint)
	}
	return labels
}

func (localizer textLocalizer) localizeMediaActions(actions *MediaGalleryActions) {
	if actions != nil {
		localizer.localizeRendererAction(actions.Upload)
		localizer.localizeRendererAction(actions.Link)
		localizer.localizeRendererAction(actions.Update)
		localizer.localizeRendererAction(actions.Reorder)
		localizer.localizeRendererAction(actions.Recenter)
		localizer.localizeRendererAction(actions.Crop)
		localizer.localizeRendererAction(actions.Remove)
	}
}

func (localizer textLocalizer) localizeMediaGalleryItems(items []MediaGalleryItem) {
	for index := range items {
		localizer.localizeMediaGalleryItem(&items[index])
	}
}

func (localizer textLocalizer) localizeMediaGalleryItem(item *MediaGalleryItem) {
	if item == nil {
		return
	}
	localizer.localizeTextFields(&item.Title, &item.Description)
	for actionIndex := range item.Actions {
		localizer.localizeRendererAction(&item.Actions[actionIndex])
	}
}

func (localizer textLocalizer) localizeMediaCropper(cropper *MediaCropperConfig) {
	if cropper == nil {
		return
	}
	localizer.localizeTextFields(&cropper.Title, &cropper.Subtitle, &cropper.Hint, &cropper.ChooseLabel, &cropper.CancelLabel, &cropper.ConfirmLabel, &cropper.CloseLabel)
}

func (localizer textLocalizer) localizeCollection(collection *CollectionConfig) {
	if collection == nil {
		return
	}
	localizer.localizeTextFields(&collection.LoadingLabel)
	for i := range collection.Actions {
		localizer.localizeRendererAction(&collection.Actions[i])
	}
	for i := range collection.Buckets {
		bucket := &collection.Buckets[i]
		localizer.localizeTextFields(&bucket.Title, &bucket.CountLabel, &bucket.AddLabel, &bucket.ClearLabel, &bucket.ModalTitle, &bucket.ModalSubtitle, &bucket.ConfirmLabel)
		for j := range bucket.Actions {
			localizer.localizeRendererAction(&bucket.Actions[j])
		}
	}
	if collection.Modal != nil {
		localizer.localizeTextFields(&collection.Modal.SearchPlaceholder, &collection.Modal.EmptyLabel, &collection.Modal.SelectedLabel, &collection.Modal.TakenLabel, &collection.Modal.CancelLabel, &collection.Modal.ConfirmLoadingLabel)
	}
}

func (localizer textLocalizer) localizeRecordPage(page *RecordPage) {
	localizer.localizeTextFields(&page.Title, &page.Subtitle, &page.Badge)
	for i := range page.Actions {
		localizer.localizeRendererAction(&page.Actions[i])
	}
	for i := range page.Sections {
		section := &page.Sections[i]
		localizer.localizeTextFields(&section.Title, &section.TitleFallback)
		localizer.localizeBlock(section.Block)
		for j := range section.Components {
			component := &section.Components[j]
			localizer.localizeTextFields(&component.ValueLabel, &component.ValueFallback, &component.MatrixLabel, &component.Title, &component.TitleFallback, &component.Subtitle, &component.SubtitleFallback)
			for index := range component.Items {
				item := &component.Items[index]
				item.Label = localizer.localizeWithFallback(item.Label, item.LabelFallback)
				item.LabelFallback = ""
			}
			if component.CollectionGroups != nil {
				for index := range component.CollectionGroups.Groups {
					group := &component.CollectionGroups.Groups[index]
					group.Label = localizer.localizeWithFallback(group.Label, group.LabelFallback)
					group.LabelFallback = ""
				}
			}
			localizer.localizeMediaGalleryItems(component.MediaItems)
		}
	}
}

func (localizer textLocalizer) localizeBlock(block *Block) {
	if block == nil {
		return
	}
	for overlayIndex := range block.Overlays {
		for badgeIndex := range block.Overlays[overlayIndex].Badges {
			localizer.localizeBadge(&block.Overlays[overlayIndex].Badges[badgeIndex])
		}
	}
}

func (localizer textLocalizer) localizeWithFallback(value string, fallback string) string {
	localized := localizer.localizeRendererText(value, "")
	if localized == value && fallback != "" {
		return fallback
	}
	return localized
}

func (localizer textLocalizer) localizeResourceGridPage(page *ResourceGridPage) {
	localizer.localizeRendererAction(page.Create)
	localizer.localizeRendererAction(page.Delete)
	localizer.localizeRendererAction(page.Update)
	if page.Card != nil {
		localizer.localizeCardSchema(page.Card)
	}
	for key, value := range page.Text {
		page.Text[key] = localizer.localizeRendererText(value, "")
	}
}
