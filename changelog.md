# Changelog

## [Unreleased]

### Added

- **`FieldPresentation.MinFieldNotice` (`min_field_notice`)** — что сказать
  под полем, когда control поднял введенное число до `min_field`: цена заказа
  ниже ставки модели становится ставкой, и человек видит почему
  (theGHub1/api#454). `{value}` — граница; текст локализуется.

- **`FieldPresentation.CalendarMarks` (`calendar_marks`)** — отмеченные дни
  календаря: поле записи с датой или списком дат, подпись для легенды и тон.
  Ожидаемая дата приезда обведена, начала других туров модели — другим цветом
  (theGHub1/api#411). Без `field` отклоняется; подписи локализуются.

- **`Media.Preview` (`preview`)** — картинка или видео карточки по нажатию
  открывается в просмотрщике вместе с остальными элементами своего поля,
  что бы ни открывала сама карточка: модератор смотрит видео верификации прямо
  из очереди (theGHub1/api#409).

- **`FieldMatrixCell.EnabledIf` (`enabled_if`)** — cell матрицы зависит от
  самой формы: пока condition не выполняется для её record (черновика), cell
  выключена и заблокирована. Глобально выключенный канал гаснет у всех типов
  сразу (theGHub1/api#405). Без `field` отклоняется; копируется глубоко.

- **`renderer.InfoHint` (`info`)** — постоянное пояснение «i» рядом с тем,
  что оно объясняет: `FieldPresentation.Info` (подпись поля в форме и в
  записи), `RecordSection.Info`, `FormSection.Info`, `DisplayComponent.Info` и
  `ListPage.Info` (заголовок страницы списка). `{id, title, text, action}`;
  без `text` отклоняется. Проверяется, копируется и локализуется вместе со
  страницей (`CloneInfoHint`, `LocalizeInfoHint`).
  `ActionPresentation.Info` — «i» рядом с действием и командой рабочего
  пространства: почему действие ждёт, что оно сделает (theGHub1/api#403).

- **`renderer.Tip` (`tips`)** — временные подсказки страницы, по одной на
  экран (`device`: `desktop` | `mobile`): `ListPage.Tips`, `RecordPage.Tips`,
  `FormPage.Tips` и `ResourceGridPage.Tips`. Ответ записывает действие
  `dismiss` producer-а, `version` говорит подсказку заново. `steps` делают
  знакомство (`next_label`, `skip_label`, `done_label`), `presentation: story`
  — историю со сценой `steps[].scene` и `back_label`; `cast` рисует читателя
  его ролью (`renderer.TipCasts`); `brand` ставит знак приложения над первой
  карточкой; `demo` — образец на странице: `media_item` (`label`, `menu`,
  `picture` — одна из картинок приложения) или `people` (`people[]`: `name`,
  `role`, `relation` `follower` | `mutual`). Одна подсказка дважды на одном
  экране, неизвестные `device`, `presentation`, `demo.kind`, `cast`, `role` и
  `relation` отклоняются.

- **`MediaGalleryItem.OriginalSrc` / `OriginalThumbnail` (`original_src`,
  `original_thumbnail`)** — картинка такой, какой её снял владелец, рядом с
  `src`, который показывает её так, как видят другие (лицо замаскировано), и
  **`MediaGalleryLabels.ViewMine` / `ViewOthers` (`view_mine`,
  `view_others`)** — подписи двух видов. Эти две подписи библиотека не
  переводит: producer передаёт готовый текст.

- **`BlockOverlay.Info` (`overlays[].info`)** — пояснение «i» рядом с бейджами
  overlay: что значат эти бейджи. Проверяется, копируется и локализуется вместе
  со страницей.

- **`MediaGalleryItem.Set` (`set`)** — файлы публикации, которую представляет
  плитка, в порядке публикации, чтобы их можно было пролистать прямо в
  галерее. Это обычные `MediaGalleryItem`; producer отдаёт их без действий.

- **`MediaGalleryActions.Open` (`media.actions.open`)** — переход от одной
  картинки к месту, где лежат все (от лица профиля к его галерее).

- **`actions.NewStatusError` / `actions.ErrorStatus`** — отказ `BeforeAction`
  или role hook-а может назвать свой 4xx (например, 404 для записи, которую
  читателю видеть нельзя); остальное по-прежнему 400. `ErrorStatus` читает
  статус и у `AtomicCommittedRejection`. Текст отказа не переводится.

- **`Generator.SetTranslationOverrides(lang, words)`** — слова, которые
  приложение заменяет во время работы (тексты, которые правит администратор):
  читаются раньше файлов в `Translate` и сливаются с ними в
  `GET /api/lang/:key`. Каждый вызов атомарно публикует новый набор языка.

- **`ModuleField.PresentationFunc` и `ModuleField.MediaFunc`** — presentation
  и media поля для текущего запроса (лимит по роли, путь к галерее того, чья
  она). Функция получает копию и возвращает то, что показать; `nil` оставляет
  поле. Применяются в `defrec` и `view`; общая metadata не меняется.

- **`ConfigNavigationEntry.LockAction` (`navigation[].lock_action`) и
  `Generator.AccessGateAction`** — шаг, который открывает закрытый пункт меню
  (верификация, недозаполненный профиль). Спрашивается только для закрытых
  пунктов navigation; тексты переводит приложение.

- **Workspace: `WorkspaceWidget.Threads` (`threads`) и `MasterVariants`
  (`master_variants`)** — строка master делится на нити, рассортированные по
  группам (`group_field`, `groups[]`, `lookup_field`/`lookup_filter`,
  `badges[]`); detail, composer и команды читают открытую нить. Pill
  `key=val` может переключить master на свой список (заказы, туры), строка
  которого разворачивается в нити (`unfold`). В `load` — `threads` и
  `master_variants`. **`WorkspaceCommand.MultiLabel` / `MultiConfirm`** —
  название multi-команды на панели выбранных строк и вопрос один раз на все
  (`{count}`, `{plural:…}`).

- **Действия: `ActionPlacementComposer` (`composer`),
  `ActionPlacementThread` (`thread`), `ActionPresentation.CountdownField`
  (`countdown_field`), `Screen` (`screen`: `desktop` | `mobile`), `Control`
  (`control: switch`)**; **`Confirm.MessageField` (`message_field`)** — текст
  вопроса из поля записи; **`Condition.Future` / `Past`** — условие по
  моменту и часам читателя.

- **Поля и варианты: `FieldPresentation.Suggest` (`suggest`: `endpoint`,
  `params`, `value_field`)**, **`MinField` / `MaxField` (`min_field`,
  `max_field`)** — концы одного диапазона в двух полях формы;
  **`ModuleFieldOptions.Group` (`group`)**, переводится как `badge`, и
  **`Exclusive` (`exclusive`)** — вариант «любой» в множественном выборе;
  renderer keys **`switch_list`** и **`segmented`**.

- **Фильтры: `Filters.Defaults` (`defaults`)** — фильтры, с которыми список
  открывается (каждый ключ должен быть доступным фильтром, иначе list
  отвечает ошибкой); **`Filters.Disclosure` (`disclosure`: `label`,
  `open`)**; **`FilterGroup.VisibleIf`** (читает `filters.<key>`);
  **`FilterPill.Icon`**; **`DateRangeConfig.OpenEndField` / `OpenEndLabel` /
  `OpenEndHint`** — диапазон без конца.

- **Карточка: `CardSchema.Segments` (`segments`: `field`, `tone_map`,
  `pulse`, `label`)**, **`CardEdgeAccent.Wash`** и tone ведущей линии из поля
  строки (`"{{field}}"`), **`Media.MoreField` (`more_field`)**,
  **`Badge.Action` (`badges[].action`)**, формат текста
  **`TextFormatHandle` (`handle`)**.

- **Формы и записи: `FormSection.VisibleIf`**, **`FieldMatrixList.DisplayType`
  / `Align`**, **`RecordSection.MobileFold`** (подпись свёрнутой на телефоне
  группы секций), **`DisplayComponent.MobileColumns` (0–4), `FormLook`,
  `HeadActions`, `Kicker`, `Highlight`** (`kicker` и `highlight` не
  переводятся), **`DisplayFieldRef.BadgeCorner`**, `display_type`
  **`check_list`** и **`progress`** (`status_timeline`) и **`plan_card`**
  (`data_list`).

- **`Confirm.Next` (`confirm.next`)** — второй вопрос, который задаётся сразу
  после «да» на первый (модель в туре подтверждает, что на месте, затем что
  передаёт анкету в управление). Действие выполняется после последнего
  ответа. Копируется, локализуется и проверяется вместе с первым.

- **`NavigationEntry.Account` (`account`)** — пункт меню, ведущий в свой
  профиль. Меню телефона рисует его как аккаунт: аватар вместо иконки и под
  ним переключатель доступности из меню аккаунта.

- **`ComponentRatioLandscape` (`main_ratio: landscape`)** — галерея рамкой 3:2:
  картинка, обрезанная под эту форму при выборе (обложка тура), стоит целиком,
  без обрезки 16:9.

- **`FieldSuggest.Optional` (`suggest.optional`)** — параметры предложения,
  которые могут быть пустыми: пустой не попадает в запрос и не держит вопрос;
  остальные параметры по-прежнему должны быть заполнены. Одно предложение может
  так читать и место, и выбранных моделей, когда что-то из них ещё не выбрано.

- **`DisplayFieldRef.Icon` (`items[].icon`)** — значок ячейки в этом компоненте
  вместо значка поля: одно поле — плитка формы в одном месте и строка со
  значком в своей панели в другом.

- **`DisplayComponent.MobileFold` (`mobile_fold`)** — компонент на телефоне
  свёрнут под заголовок, который его открывает; соседние компоненты с тем же
  `mobile_fold` открываются и закрываются вместе, заголовок — `title` первого
  из них. Широкий экран показывает их как есть.

- **`display_type: progress_rows`** для `record_carousel` — строки записей,
  каждая на своём пути по одному ряду шагов: где она сейчас (`badge`), что это
  значит в днях и датах (`note`), полоса шагов (`progress`: `steps`,
  `reached`, `fraction`, `stopped`) и шаг читателя (`action_ids`) в конце.

- **`FieldMatrixList.MobileColumns` (`mobile_columns`)** — сколько items списка
  стоит в ряду на телефоне (1–4); без него решает consumer.

- **`CardSchema.Chips` (`chips`)** — набор коротких значений (например, страны
  профиля) строкой под subtitle: иконка в первой плашке, «+N» со списком всех.

- **`FieldMediaConfig.Capture` (`media.capture`)** — съёмка камерой устройства
  для media field: фото или видео, рамка лица (`face`) или фигуры в полный
  рост (`body`), таймер, минимальная и максимальная длина видео, предложение
  продолжить на телефоне по QR-коду. Снятое уходит в тот же `upload`.
  Видео может идти по шагам (`steps`: рамка, `props` — листок и/или речь,
  подсказка, секунды) с отсчётом перед записью, подсказкой следующего шага
  (`next_step_label`) и словами после съёмки (`done_title`, `done_text`).

- **`WidgetSurface.PinnedRoutes`** — страницы, на которых виджет нельзя
  закрыть и он виден, даже если его закрыли в другом месте (путь с `*` на
  конце — префикс). JSON: `pinned_routes`.

- Compact `renderer.Discovery` and `BaseModule.DiscoveryFunc` for config route
  capabilities without cloning page contents. Existing discovery/runtime hooks,
  validation and per-request access gates remain supported; wire format unchanged.
- `renderer.LocalizeOwned` for request-owned renderer graphs. Page responses no
  longer deep-clone the whole renderer again for localization; aliased text is
  visited once. `renderer.Localize` retains its copying contract. Producers must
  keep newly attached mutable callback results request-owned.
- Field media localization makes one copy instead of two.

- **`DisplayPrompts` (`prompts`) и `DisplayComponent.Prompts`** — уведомления
  записи с шагом, который на них отвечает, в любом месте секции страницы
  записи. Тот же `PromptList`, что у секции формы. JSON:
  `components[].prompts`.

- **`Prompt.Attention`** — уведомление, которое ждёт ответа, отмечено бегущим
  по краю светом. JSON: `attention`.

- **Result fields atomic actions в action result contract** — скалярные
  `ResultFields` atomic `add`/`update` можно назвать в
  `selection.source.field`, а endpoint вида `/api/items/id/{id}` совпадает со
  standard request `/api/items/:bykey/:value`.

- **`AtomicExecutor.SelectMany`** — ограниченное и детерминированное typed
  чтение нескольких строк внутри generator-owned transaction. Контракт требует
  `ORDER BY` и положительный `LIMIT`; пустой результат не является ошибкой.

- **`AtomicExecutor.Update`** — typed обновление строк с обязательным Jet
  `Where`; закрытые операции `set` и numeric `increment` не допускают
  произвольных SQL expressions. Добавлен `AtomicTime` для timestamp values.

- **`AtomicExecutor.Upsert` и atomic realtime publish** — конфликтобезопасное
  идемпотентное создание без доступа модуля к transaction/raw SQL, а также
  typed публикация actor-scoped realtime-события только после commit.
  Recipient topics строятся generator-ом из server-produced result fields.

- **`NavigationEntry` на `BaseModule`** — декларативное описание навигации и frontend routes для config endpoint.
  Поля: `ActionName`, `ID`, `Path`, `Title`, `Icon`, `Show`, `Order`, `Group`, `Target`, `Roles`, `Query`, `Data`.

- **Config endpoint** — возвращает единый role-based список `navigation`, глобальные `widgets` и роль пользователя.
  Навигация формируется из `Navigation` каждого модуля, фильтруется по правам действия и `Roles` пункта.
  Для `target.type=page` renderer/query/children встраиваются прямо в `navigation[].target`, без отдельного `routes`.

- **`WidgetConfig` на действиях модулей** — глобальные виджеты описываются на конкретном действии (`list`, `view`, `add`, `defrec`, ...).
  Такие виджеты автоматически попадают в `/api/config.widgets` с query соответствующего действия.

- **`ExtraFunc` на `ListModuleAction`** — динамические extra-данные per-request.
  Функция вызывается при каждом List-запросе; результат добавляется в ответ как `extra`.

  ```go
  ExtraFunc: func(c *gin.Context) interface{} {
      return map[string]interface{}{"pills": buildPills(c)}
  }
  ```

- **`FilterFunc` на `ListModuleAction`** — динамический список фильтруемых колонок per-request.
  Заменяет/дополняет статический `Filter []pg.Column` когда набор доступных фильтров зависит от контекста запроса.

  ```go
  FilterFunc: func(c *gin.Context) []pg.Column {
      // вернуть нужные колонки на основе c
  }
  ```

- **`FilterCondition` на `ModuleField`** — функция условия видимости поля в фильтрах.
  Если задана — поле включается в фильтры только когда функция возвращает `true`.

  ```go
  {
      Column:          table.Courses.Price,
      FilterCondition: func(c *gin.Context) bool { ... },
  }
  ```

- **`Where` на `UpdateModuleAction`** — дополнительное WHERE-условие для UPDATE.
  Если ни одна строка не обновлена — возвращается 404. Возврат `nil` снимает ограничение.

  ```go
  actions.UpdateModuleAction{
      Where: func(c *gin.Context) pg.BoolExpression {
          // вернуть условие или nil
      },
  }
  ```

- **`Where` на `DeleteModuleAction`** — дополнительное WHERE-условие для DELETE.
  Если ни одна строка не удалена — возвращается 404.

  ```go
  actions.DeleteModuleAction{
      Where: func(c *gin.Context) pg.BoolExpression {
          // вернуть условие или nil
      },
  }
  ```

- **`Group` и `Order` на `ModuleField`** — организация полей в группы фильтров.
  `Group` — строковый ключ группы. `Order` — порядок внутри группы.
  Оба поля экспортируются в JSON при `addFilters=true`.

  ```go
  {
      Column: table.Courses.Status,
      Group:  "options",
      Order:  1,
  }
  ```

- **`DataCheckRule` interface** — валидация с доступом к БД и контексту запроса.
  Встраивает `CheckRules` (обратная совместимость) и добавляет `ValidateData`. Генератор автоматически определяет реализацию и вызывает `ValidateData` вместо `Validate`.

  ```go
  type DataCheckRule interface {
      CheckRules
      ValidateData(c *gin.Context, db *sql.DB, data map[string]interface{}, lang string) error
  }
  ```

  Реализация через конструктор `DataRule()` или собственный тип:

  ```go
  // Inline:
  fields.DataRule(func(c *gin.Context, db *sql.DB, data map[string]interface{}, lang string) error {
      // доступ к data["field"], выполнение DB-запросов
      return nil
  }, []fields.Scenario{fields.ScenarioAdd})

  // Собственный тип:
  type myRule struct{ scenarios []fields.Scenario }
  func (r myRule) Validate(_ interface{}, _ string) error { return nil }
  func (r myRule) GetScenarios() []fields.Scenario       { return r.scenarios }
  func (r myRule) ValidateData(c *gin.Context, db *sql.DB, data map[string]interface{}, lang string) error {
      // логика
      return nil
  }
  ```

- **`DefaultFunc` защита от spoofing** — поля, не входящие в `action.Columns`, теперь всегда получают значение через `DefaultFunc(c)`, даже если клиент передал то же поле в теле запроса. Это предотвращает подмену системных полей (`who_add`, `user_id` и аналогичных).

- **`RawDB()` на `DBExecutor`** — новый метод интерфейса возвращает `*sql.DB` для использования в `DataCheckRule.ValidateData`.

  ```go
  type DBExecutor interface {
      // ...
      RawDB() *sql.DB
  }
  ```

- **`SetDB` / `GetDB` в `icontext`** — утилиты для передачи `*sql.DB` через `context.Context`.

- **`DateRangeConfig.MinDays` и `MinDaysLabel`** — минимальная длина диапазона
  дат в днях (обе границы включительно) и локализуемое пояснение к ней.
  Picker не даёт собрать диапазон, который сервер потом отклонит.
  JSON: `date_range.min_days`, `date_range.min_days_label`.

  ```go
  DateRange: &renderer.DateRangeConfig{
      StartField:   "starts_on",
      EndField:     "ends_on",
      MinDays:      3,
      MinDaysLabel: "booking.min_days_hint",
  }
  ```

- **`StatusBinding.LabelMap`** — названия состояний статуса карточки, когда
  строка списка несёт только сырое значение. JSON: `card_schema.status.label_map`.

- **`Badge.IconOnly`** — бейдж, который показывается только иконкой.
  JSON: `icon_only`.

- **`DisplayComponent.ShowEmpty`** — блок показывает объявленные поля, даже
  если у записи ещё нет значений. JSON: `show_empty`.

- **Новые `ActionPlacement`: `ActionPlacementHead`, `ActionPlacementMenu`,
  `ActionPlacementHalf`** — действие рядом со строкой identity карточки
  (`head`), в подписанной группе действий (`menu`) или на половину строки
  (`half`). Подпись группы задаёт **`CardSchema.ActionMenuLabel`**
  (`action_menu_label`, локализуется).

- **Новые поля `ActionPresentation`:**
  - `ActiveIf` (`active_if`) — условие «это текущий выбор» для набора
    взаимоисключающих действий; пустое условие отклоняется;
  - `Description` / `DescriptionKey` (`description`, `description_key`) —
    строка пояснения под label, локализуется как `label`;
  - `ValueField` / `ValueIcon` (`value_field`, `value_icon`) — значение поля
    записи рядом с label и иконка перед ним;
  - `AttentionKey` (`attention_key`) — действие выделяется, пока его не
    использовали; renderer запоминает использование под этим ключом.

- **`Action.AfterFailure`** (`ActionFailure`) — что предложить, когда операцию
  отказали: `title`, `cancel_label`, `confirm_label` и `route`. Текст отказа
  приходит из API. Поле локализуется и копируется вместе с action.
  JSON: `after_failure`.

- **`ActionResult.NextAction`** — id действия той же страницы, которое
  запускается после успеха текущего. Для `form_page.actions` generator
  отклоняет необъявленный id и ссылку действия на само себя.
  JSON: `after_success.next_action`.

  ```go
  AfterSuccess: &renderer.ActionResult{NextAction: "save_template"}
  ```

- **`Prompt.CloseLabel`** — локализуемая подпись второго ответа на prompt
  («позже»). JSON: `prompts.items[].close_label`.

- **Новые поля `FieldPresentation`:**
  - `Placeholder` (`placeholder`) — локализуемый текст внутри пустого control;
  - `RequiredIf` (`required_if`) — поле обязательно только в указанном
    состоянии;
  - `DisabledIf` (`disabled_if`) — в указанном состоянии поле видно, но
    недоступно для изменения;
  - `NoticeByValue` (`notice_by_value`, `[]FieldValueNotice`) — уведомление
    при выборе значения: `value`, `title`, `message`, `confirm_label`
    (тексты локализуются в `defrec` и `view`).

- **`ModuleField.TitleFunc`** — заголовок поля для текущего запроса в `defrec`.
  Функция возвращает translation key; пустой результат оставляет `Title`.

  ```go
  TitleFunc: func(c *gin.Context) string {
      if recordIsOrganization(c) {
          return "records.fields.organization_title"
      }
      return ""
  },
  ```

- **`ModuleFieldOptions.Media`, `Badge`, `BadgeTone`, `Trailing`,
  `TrailingNote`, `Note`** — вариант выбора может быть картинкой, а вариант-
  карточка может нести метку, trailing-значение с подписью и строку акцента.
  `badge`, `note` и `trailing_note` переводятся (в `defrec`, `view` и обычных
  list filters). Вариант без этих полей сериализуется как раньше.

- **`min`/`max` в `defrec.fields[field]`** — принимаемый диапазон из check
  rule, у которого `RuleInfo().Type == "range"` (сценарий `add`).

- **Новые поля `Media` (card media):** `CountField` (`count_field`, число на
  ободке картинки), `MarkerField` / `MarkerIcon` (`marker_field`,
  `marker_icon`, метка в углу картинки), `FallbackField` (`fallback_field`,
  значение записи вместо отсутствующей картинки).

- **`MediaRatioNatural` (`natural`) и `MediaSizeOriginal` (`original`)** —
  картинка сохраняет исходную форму или запрашивается файлом в том виде, в
  каком её загрузили.

- **`MediaUsageCover` (`cover`)** — назначение media «обложка карточки».

- **`MediaGalleryItem.OpenAction`, `Cover`, `PostCount`** — что открывает тап по
  элементу галереи (typed `Action`, обязателен `type`), признак обложки всего
  набора и число публикаций с этой картинкой. JSON: `open_action`, `cover`,
  `post_count`.

- **`MediaGalleryActions.SetAvatar` и `SetCover`** — сделать существующую
  картинку аватаром или обложкой. JSON: `media_actions.set_avatar`,
  `media_actions.set_cover`.

- **Новые подписи `MediaGalleryLabels`:** `FilterAll`, `FilterPublic`,
  `FilterPrivate`, `FilterHidden`, `FilterVideo`, `FilterUnpublished` (части
  галереи), `Hidden`, `HiddenHint`, а также `More`, `ViewAll`, `Close` (для
  ограниченной полосы миниатюр).

- **`MediaUploadConfig.MinDurationSeconds` и `MinDurationError`** —
  минимальная длина видео до загрузки и локализуемый текст отказа.
  JSON: `min_duration_seconds`, `min_duration_error`.

- **`FormSection.MediaVisibilityStates`** (`MediaVisibilityOption`: `value`,
  `label`, `icon`, `hint`, `confirmation`) — состояния видимости, между
  которыми можно переводить элемент галереи. Generator проверяет, что `value`
  известен, `label` задан и значения не повторяются.
  JSON: `media_visibility_states`.

- **`FormSection.MediaCropper`** — cropper галереи для ролей со своей формой
  (например, круглый аватар). JSON: `media_cropper`.

- **`FormSection.MediaPresets`** (`MediaPresetsConfig`: `title`, `subtitle`,
  `show_label`, `hide_label`, `add_label`) — предложение готовых картинок для
  галереи. JSON: `media_presets`.

- **`FormSection.Sections`** — вложенные блоки form-секции. Generator
  разрешает их `Resource`/`matrix.source` и локализует тексты так же, как у
  родителя. JSON: `form_page.sections[].sections`.

- **`RecordSection.Subtitle`** — локализуемая строка под заголовком
  record-секции. JSON: `record_page.sections[].subtitle`.

- **`Block.Icon` и `Block.Decoration`** (`BlockDecoration`: `source`,
  `variant`) — иконка в заголовке панели и декоративная картинка. Варианты:
  `BlockDecorationCornerWide`, `BlockDecorationCornerSquare`,
  `BlockDecorationCornerFloating`, `BlockDecorationLeadingBanner`,
  `BlockDecorationBackground`, `BlockDecorationInline`.

- **`DisplayComponent.Preview`** (`DisplayPreview`: `label`, `close_label`,
  `actions`) — картинку компонента `identity` или `media_gallery` можно открыть
  на весь экран вместе с действиями из `record_page.actions`. Generator
  отклоняет пустые, повторяющиеся и необъявленные id действий.

- **`DisplayComponent.ThumbLimit`, `ThumbLimitWide`, `MediaLabels`** — сколько
  миниатюр `media_gallery` показывать до «ещё» (обычно и на широком экране) и
  подписи галереи внутри компонента. Отрицательные значения и положительные
  значения у компонентов других типов отклоняются.

- **`DisplayComponent.AutoScroll`** — полоса карточек медленно прокручивается
  сама и останавливается под указателем. JSON: `auto_scroll`.

- **`DisplayComponent.ItemFilter`** (`ItemFilter`, `ItemFilterOption`) —
  компонент сужает свои элементы поиском и выбором состояния.
  JSON: `item_filter`.

- **`DisplayComponent.ItemSelection`** (`ItemSelection`) — выбор нескольких
  элементов набора с суммой по `amount_field` и передачей их id в действие
  страницы под именем `ids_key`. JSON: `item_selection`.

- **`DisplayRecordCarousel` (`record_carousel`)** — тип компонента для набора
  записей.

- **Новые `ComponentDisplayType`: `ComponentDisplayActionRows`
  (`action_rows`), `ComponentDisplayFlowSteps` (`flow_steps`),
  `ComponentDisplayCardRail` (`card_rail`)** — действия строками с пояснением,
  нумерованные шаги и полоса узких карточек.

- **`ComponentRatioTall` (`tall`)** — сетка картинок из высоких плиток.

- **`FilterPillPresentationMenu` (`menu`)** — pills одного ряда показываются
  одним меню с выбранным вариантом.

- **`ResourceGridPage.HeadActions`** — дополнительные действия рядом с
  `create` в голове resource grid; проверяются, локализуются и копируются как
  остальные действия страницы. JSON: `resource_grid_page.head_actions`.

- **`WorkspaceWidget.ComposerBadges` и `RetryLabel`** — бейджи над composer
  (`id` обязателен и уникален) и локализуемая подпись повтора неудачного
  запроса. JSON: `workspace.composer_badges`, `workspace.retry_label`.

- **`sort_active` в list response** (`actions.SortActiveResponse`: `field`,
  `direction`) — сортировка, в которой строки реально отданы. Ключа нет, если
  list action не сортирует.

- **`Generator.AccessGate`** (`AccessGate`, `AccessTarget`) — приложение
  закрывает пункт навигации, глобальный виджет или route для текущего актора,
  не убирая его из конфигурации. `/api/config` отдаёт `locked` и
  `lock_reason` у `navigation[]`, `widgets[]` и `routes[]`.

  ```go
  generator.AccessGate = func(c *gin.Context, target module.AccessTarget) (bool, string) {
      // target.Kind: "navigation", "widget" или "route"
      return false, ""
  }
  ```

- **`Generator.NavigationHidden`** — пункт навигации, которого для актора нет
  вообще, убирается из `/api/config.navigation` (lock означает «пока нет»,
  hidden — «не для тебя»).

- **`NavigationEntry.Home`** — пункт, на который ведёт бренд для этого актора.
  JSON: `navigation[].home`.

- **`module.RealtimeUserTopic`** — публичное имя топика `user:{id}` для
  публикаций вне atomic pipeline.

- **`AtomicValueKindNullableInt`** — atomic select читает nullable integer:
  для `NULL` возвращается пустой `AtomicValue` вместо ошибки.

- **`AtomicUpdateClear`** — atomic update очищает колонку (`NULL`) для
  timestamptz, timestamp, date, string, integer и float. Для остальных типов
  возвращается ошибка.

  ```go
  actions.AtomicUpdateField{Column: table.Records.ArchivedAt, Operation: actions.AtomicUpdateClear}
  ```

- **`AtomicExecutor.Delete`** (`AtomicDelete`) — удаление строк по
  обязательному Jet `Where` в той же транзакции, что и остальные atomic
  записи. Без `Table` или `Where` SQL не выполняется.

  ```go
  deleted, err := executor.Delete(ctx, actions.AtomicDelete{
      Table: table.Drafts,
      Where: table.Drafts.ID.EQ(pg.Int(id)),
  })
  ```

- **`AtomicExecutor.Update` для JSON и `text[]`** — операция `set` передаёт
  `AtomicValue.JSON` и `AtomicValue.Strings` в string-колонку нетипизированным
  параметром, как это делает insert. Поэтому их принимают колонки `jsonb` и
  `text[]`.

### Changed

- **`TitleFunc` действует и во `view`** — `view.item[field].title` теперь
  тоже берёт заголовок из `TitleFunc`: форма, которая читает запись через view,
  называла переключатель TopMember-а словами модели (theGHub1/api#367). List
  `heads` по-прежнему берут `Title`.

- **Отказы hook-ов и `BeforeAction`** отвечают статусом, который называет
  ошибка (`actions.ErrorStatus(err, 400)` во всех шести местах
  `BeforeRequest` и пяти местах role hook-ов); ошибка без статуса — 400, как
  раньше.

- **`Convert` на `ModuleField`** принимает `*gin.Context` первым аргументом.
  Это позволяет использовать роль, пользователя или другие данные контекста при преобразовании значения.

  ```go
  // Было:
  Convert: func(value interface{}) (interface{}, error) { ... }

  // Стало:
  Convert: func(c *gin.Context, value interface{}) (interface{}, error) { ... }
  ```

- `NavigationEntry` поддерживает поля `Query` (`map[string]interface{}`) и `Data` (`map[string]interface{}`) для передачи дополнительных параметров клиенту.
- `NavigationEntry.Path` позволяет явно задать frontend route для `target.type=page`.

- **Версия `UniversalRenderer`: `2.6.0` → `2.7.0`** (`renderer.Version` и
  паспорт `docs/universal-renderer-contract.md`) — minor, потому что все
  изменения wire-формата выше аддитивны: новые optional поля, новые значения
  закрытых enum, которые producer включает явно, и новые optional ключи ответов
  (`sort_active`, `fields[field].min`/`max`, `locked`, `home`). Required keys,
  CRUD route mapping и error shape не менялись. Версия также покрывает поля,
  добавленные после `2.6.0` без смены версии (`navigation[].mobile_order`,
  `mobile_title`, `navigation_more_label`, `layout.mobile_slots`,
  `sections[].mobile_order`, `form_page.navigation`, `RecordSection.Resource`,
  `date_range.min_days`). Новые значения закрытых enum producer может отдавать
  только после обновления consumer-а.

- **`DisplayComponent.DisplayType` проверяется по типу компонента** — каждое
  значение допустимо только для своего типа: `key_value_grid`/`tile_grid` —
  `data_list`, `action_rows` — `actions`, `flow_steps` — `status_timeline`,
  `card_rail` — `record_carousel`. Для уже существующих значений поведение не
  изменилось; изменился текст ошибки
  (`display type "…" requires component type "…"`).

- **`actions.AtomicExecutor` получил метод `Delete`** — собственные реализации
  интерфейса (в том числе test doubles) должны его добавить.

  ```go
  type AtomicExecutor interface {
      // ...
      Delete(context.Context, AtomicDelete) (int64, error)
  }
  ```

### Fixed

- **Фильтр, выбор и медиа компонента при переводе** — `item_filter`,
  `item_selection` и `media_items` display component-а копируются вместе со
  страницей. Раньше перевод первого читателя записывался в страницу producer-а,
  и следующие читатели видели её на его языке.

- **Клонирование `FieldMatrixList`** — список копируется целиком, поэтому
  `display_type` и `align` больше не теряются по дороге к ответу.

- **`back_label` истории** переводится вместе с остальными подписями
  подсказки; второй вопрос `Confirm.Next` переводится вместе с первым.

- **Фильтр по array-колонке с одним значением** — одиночное значение (например,
  от pill) оборачивается в Postgres array literal `{value}`. Раньше список
  отвечал ошибкой `malformed array literal`.

- **Запись, возвращаемая после update** — поля с request-scoped projection
  теперь разрешаются через `fields.ResolveProjections` и при чтении итоговой
  записи (через view action и в fallback). Раньше update отвечал ошибкой view,
  потому что запрос обращался к несуществующей колонке.

- **Форматирование `module.go` и `renderer/tokens.go`** — файлы приведены к
  `gofmt`; `gofmt -l` больше их не показывает.
