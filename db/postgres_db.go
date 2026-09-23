package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

// sqlIdentifierPattern matches a bare SQL identifier: a table alias or column
// name a producer declared, never an expression assembled from a request.
var sqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// SafeSQLIdentifier reports whether name may be pasted into SQL as an
// identifier. A dotted filter key's table part is the one place a caller can
// steer that text, so it is checked here before it reaches a query.
func SafeSQLIdentifier(name string) bool {
	return sqlIdentifierPattern.MatchString(name)
}

type DB struct {
	DBExecutor
	sql           *sql.DB
	Debug         bool
	preparedViews *preparedViewCache
	readContext   context.Context
}
type Tx struct {
	sql *sql.Tx
}

func NewDB(sql *sql.DB) *DB {
	return &DB{
		sql: sql,
	}
}

func (db *DB) debugLog(log *log.Entry, args ...interface{}) {
	if db.Debug {
		log.Infoln(args...)
	}
}

func interpolateQuery(query string, args []interface{}) string {
	for i := len(args); i >= 1; i-- {
		placeholder := fmt.Sprintf("$%d", i)
		query = strings.Replace(query, placeholder, fmt.Sprintf("'%v'", args[i-1]), 1)
	}
	return query
}

func (db *DB) Begin() (*Tx, error) {
	tx, err := db.sql.Begin()
	if err != nil {
		return nil, err
	}
	return &Tx{tx}, nil
}

func moduleFieldResultValue(field fields.ModuleField, value interface{}) interface{} {
	if field.ResultValueConverter != nil {
		return field.ResultValueConverter(value)
	}

	raw := value
	if valuer, ok := value.(driver.Valuer); ok {
		resolved, err := valuer.Value()
		if err != nil {
			return nil
		}
		raw = resolved
	}

	if raw == nil {
		return nil
	}

	if field.Type == fields.ModuleFieldTypeArray || field.Type == fields.ModuleFieldTypeObject {
		if parsed, ok := parseJSONResultValue(raw, field.Type); ok {
			return parsed
		}
	}

	return raw
}

func parseJSONResultValue(value interface{}, fieldType fields.ModuleFieldType) (interface{}, bool) {
	var raw string
	switch typed := value.(type) {
	case string:
		raw = typed
	case []byte:
		raw = string(typed)
	case json.RawMessage:
		raw = string(typed)
	default:
		return nil, false
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	if fieldType == fields.ModuleFieldTypeArray && !strings.HasPrefix(raw, "[") {
		return nil, false
	}
	if fieldType == fields.ModuleFieldTypeObject && !strings.HasPrefix(raw, "{") {
		return nil, false
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

func (db *DB) RowExists(query string, args ...interface{}) bool {
	var exists bool
	query = fmt.Sprintf("SELECT exists (%s)", query)
	_ = db.sql.QueryRow(query, args...).Scan(&exists)
	return exists
}

// applyJoin applies a single join to the FROM clause
func applyJoin(from pg.ReadableTable, join actions.ModuleActionJoin) pg.ReadableTable {
	switch join.Type {
	case actions.JoinTypeRight:
		return from.RIGHT_JOIN(join.Table, join.OnCondition)
	case actions.JoinTypeInner:
		return from.INNER_JOIN(join.Table, join.OnCondition)
	default:
		return from.LEFT_JOIN(join.Table, join.OnCondition)
	}
}

// buildJoinProjection builds a json_agg(json_build_array(...)) projection for a join's columns
func buildJoinProjection(join actions.ModuleActionJoin) pg.Projection {
	colRefs := make([]string, 0, len(join.Columns))
	for _, col := range join.Columns {
		colRefs = append(colRefs, fmt.Sprintf(`%s."%s"`, join.ResultArrayName, col.Name()))
	}
	rawExpr := fmt.Sprintf("json_agg(json_build_array(%s))", strings.Join(colRefs, ", "))
	return pg.Raw(rawExpr)
}

// isTranslatableField checks if a column name matches a translatable field in the context.
func isTranslatableField(tc *TranslationContext, colName string) bool {
	if tc == nil {
		return false
	}
	for _, f := range tc.Fields {
		if f.FieldName == colName {
			return true
		}
	}
	return false
}

// buildTranslationSubquery builds a subquery expression for a translatable field.
func buildTranslationSubquery(tc *TranslationContext, tableRef string, pkName string, fieldName string) pg.Projection {
	subquery := fmt.Sprintf(
		`(SELECT json_object_agg(t.lang, t.value) FROM translations t WHERE t.entity = '%s' AND t.entity_id = %s."%s" AND t.field = '%s')`,
		tc.EntityName, tableRef, pkName, fieldName,
	)
	return pg.Raw(subquery)
}

// InsertTranslations inserts translation rows within an existing transaction.
// It is used by both standard and atomic add paths; modules never receive tx.
func InsertTranslations(tx *sql.Tx, tc *TranslationContext, entityID int64, moduleFields []fields.ModuleField, input map[string]interface{}) error {
	for _, field := range moduleFields {
		if !field.Translatable {
			continue
		}
		fieldName := field.Name()
		langMapRaw, ok := input[fieldName]
		if !ok {
			continue
		}
		langMap, ok := langMapRaw.(map[string]interface{})
		if !ok {
			continue
		}
		for lang, val := range langMap {
			valStr := fmt.Sprintf("%v", val)
			_, err := tx.Exec(
				`INSERT INTO translations (entity, entity_id, field, lang, value) VALUES ($1, $2, $3, $4, $5)`,
				tc.EntityName, entityID, fieldName, lang, valStr,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// UpsertTranslations updates translation rows within an existing transaction.
// It is used by both standard and atomic update paths; modules never receive tx.
func UpsertTranslations(tx *sql.Tx, tc *TranslationContext, entityID interface{}, moduleFields []fields.ModuleField, input map[string]interface{}) error {
	for _, field := range moduleFields {
		if !field.Translatable {
			continue
		}
		fieldName := field.Name()
		langMapRaw, ok := input[fieldName]
		if !ok {
			continue
		}
		langMap, ok := langMapRaw.(map[string]interface{})
		if !ok {
			continue
		}
		for lang, val := range langMap {
			valStr := fmt.Sprintf("%v", val)
			_, err := tx.Exec(
				`INSERT INTO translations (entity, entity_id, field, lang, value) VALUES ($1, $2, $3, $4, $5)
				 ON CONFLICT (entity, entity_id, field, lang) DO UPDATE SET value = EXCLUDED.value`,
				tc.EntityName, entityID, fieldName, lang, valStr,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// fieldMeta tracks whether a field is translatable and its result key name.
type fieldMeta struct {
	translatable bool
	name         string
}

// arrayElementType determines whether an array literal contains integers or text.
// "{1,2,3}" → "integer", "{en,ru,fr}" → "text"
func arrayElementType(value string) string {
	stripped := strings.Trim(value, "{}")
	for _, part := range strings.Split(stripped, ",") {
		if _, err := strconv.Atoi(strings.TrimSpace(part)); err != nil {
			return "text"
		}
	}
	return "integer"
}

func dateRangeBounds(value string) (time.Time, time.Time, bool, bool, bool) {
	parts := strings.SplitN(value, "..", 2)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, false, false, false
	}

	fromRaw := strings.TrimSpace(parts[0])
	toRaw := strings.TrimSpace(parts[1])
	var from, to time.Time
	var hasFrom, hasTo bool
	if fromRaw != "" {
		parsed, ok := parseDateRangeBound(fromRaw, false)
		if !ok {
			return time.Time{}, time.Time{}, false, false, false
		}
		from = parsed
		hasFrom = true
	}
	if toRaw != "" {
		parsed, ok := parseDateRangeBound(toRaw, true)
		if !ok {
			return time.Time{}, time.Time{}, false, false, false
		}
		to = parsed
		hasTo = true
	}
	return from, to, hasFrom, hasTo, hasFrom || hasTo
}

func parseDateRangeBound(value string, endOfDay bool) (time.Time, bool) {
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		if endOfDay {
			return parsed.AddDate(0, 0, 1), true
		}
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func (db *DB) List(
	log *log.Entry,
	table pg.Table,
	primaryKey pg.Column,
	moduleFields []fields.ModuleField,
	filterRegistry map[string]fields.ModuleFilterField,
	page int64,
	size int64,
	searchColumns []pg.Column,
	searchText string,
	filter map[string]string,
	where pg.BoolExpression,
	joins []actions.ModuleActionJoin,
	sort *actions.SortOption,
	tc *TranslationContext,
) (result []interface{}, rowsCount int64, err error) {

	tableRef := table.TableName()
	pkName := primaryKey.Name()

	// Build projections: primary key + field columns + join aggregations
	projections := []pg.Projection{primaryKey}
	metas := make([]fieldMeta, len(moduleFields))

	for i, field := range moduleFields {
		if field.Translatable && tc != nil {
			projections = append(projections, buildTranslationSubquery(tc, tableRef, pkName, field.Name()))
			metas[i] = fieldMeta{translatable: true, name: field.Name()}
		} else {
			projections = append(projections, field.GetProjection())
			metas[i] = fieldMeta{translatable: false, name: field.ColumnName()}
		}
	}
	for _, join := range joins {
		if len(join.Columns) > 0 {
			projections = append(projections, buildJoinProjection(join))
		}
	}

	// Build FROM clause with joins
	var from pg.ReadableTable = table
	for _, join := range joins {
		from = applyJoin(from, join)
	}

	// Build WHERE conditions
	var conditions []pg.BoolExpression
	if where != nil {
		conditions = append(conditions, where)
	}

	// Search
	if len(searchText) > 0 && len(searchColumns) > 0 {
		searchConds := make([]pg.BoolExpression, 0, len(searchColumns))
		for _, col := range searchColumns {
			if tc != nil && isTranslatableField(tc, col.Name()) {
				// Search in translations table
				searchConds = append(searchConds,
					pg.RawBool(
						fmt.Sprintf(`EXISTS (SELECT 1 FROM translations t WHERE t.entity = '%s' AND t.entity_id = %s."%s" AND t.field = '%s' AND LOWER(t.value) LIKE '%%' || #search || '%%')`,
							tc.EntityName, tableRef, pkName, col.Name()),
						pg.RawArgs{"#search": strings.ToLower(searchText)},
					),
				)
			} else {
				searchTableRef := columnTableRef(col, tableRef)
				searchConds = append(searchConds,
					pg.RawBool(
						fmt.Sprintf(`LOWER(%s."%s"::text) LIKE '%%' || #search || '%%'`, searchTableRef, col.Name()),
						pg.RawArgs{"#search": strings.ToLower(searchText)},
					),
				)
			}
		}
		conditions = append(conditions, pg.OR(searchConds...))
	}

	// Filters
	if len(filter) > 0 {
		for key, value := range filter {
			parts := strings.Split(key, ".")
			colName := key
			tblRef := tableRef
			if len(parts) > 1 {
				// A dotted filter key names a joined table before the column.
				// That table reference is pasted into SQL, so it must be a
				// plain identifier - never an expression a caller assembled.
				if !SafeSQLIdentifier(parts[0]) {
					continue
				}
				colName = parts[1]
				tblRef = parts[0]
			}

			definition, ok := filterRegistry[key]
			if !ok {
				// Dotted relation filters retain the existing physical-key path.
				definition, ok = filterRegistry[colName]
			}
			if !ok || definition.Column == nil {
				continue
			}
			// A logical filter key may point at a joined column through the typed
			// registry. Preserve an explicitly dotted request target, otherwise
			// qualify the predicate with the declared column table.
			if len(parts) == 1 {
				tblRef = columnTableRef(definition.Column, tblRef)
			}
			colName = definition.Column.Name()
			ft := definition.Type
			fmt2 := definition.FormType

			// 1. Array-type columns → overlap operator (&&)
			if ft == fields.ModuleFieldTypeArray {
				castType := arrayElementType(value)
				conditions = append(conditions,
					pg.RawBool(
						fmt.Sprintf(`%s."%s" && #%s_val::%s[]`, tblRef, colName, colName, castType),
						pg.RawArgs{fmt.Sprintf("#%s_val", colName): value},
					),
				)
			} else if fmt2 == fields.ModuleFieldFormTypeMultiselect {
				// 2. Multiselect on non-array column → IN clause
				// Value is in PG array literal: {french,german}
				stripped := strings.Trim(value, "{}")
				items := strings.Split(stripped, ",")
				if len(items) == 1 {
					conditions = append(conditions,
						pg.RawBool(
							fmt.Sprintf(`%s."%s" = #%s_val`, tblRef, colName, colName),
							pg.RawArgs{fmt.Sprintf("#%s_val", colName): items[0]},
						),
					)
				} else {
					placeholders := make([]string, len(items))
					args := pg.RawArgs{}
					for i, item := range items {
						ph := fmt.Sprintf("#%s_val_%d", colName, i)
						placeholders[i] = ph
						args[ph] = item
					}
					conditions = append(conditions,
						pg.RawBool(
							fmt.Sprintf(`%s."%s" IN (%s)`, tblRef, colName, strings.Join(placeholders, ", ")),
							args,
						),
					)
				}
			} else if strings.Contains(value, "..") {
				// 3. Date/datetime range: "2026-06-01..2026-06-29" → >= start AND < next day
				from, to, hasFrom, hasTo, ok := dateRangeBounds(value)
				if !ok {
					continue
				}
				var rangeConds []pg.BoolExpression
				if hasFrom {
					ph := fmt.Sprintf("#%s_from", colName)
					rangeConds = append(rangeConds, pg.RawBool(
						fmt.Sprintf(`%s."%s" >= %s`, tblRef, colName, ph),
						pg.RawArgs{ph: from},
					))
				}
				if hasTo {
					ph := fmt.Sprintf("#%s_to", colName)
					rangeConds = append(rangeConds, pg.RawBool(
						fmt.Sprintf(`%s."%s" < %s`, tblRef, colName, ph),
						pg.RawArgs{ph: to},
					))
				}
				if len(rangeConds) > 0 {
					conditions = append(conditions, pg.AND(rangeConds...))
				}
			} else if fmt2 == fields.ModuleFieldFormTypeNumber && strings.Contains(value, "-") {
				// 4. Number range: "18-20" → >= 18 AND <= 20
				rangeParts := strings.SplitN(value, "-", 2)
				minVal := strings.TrimSpace(rangeParts[0])
				maxVal := strings.TrimSpace(rangeParts[1])
				var rangeConds []pg.BoolExpression
				if minVal != "" {
					ph := fmt.Sprintf("#%s_min", colName)
					rangeConds = append(rangeConds, pg.RawBool(
						fmt.Sprintf(`%s."%s" >= %s`, tblRef, colName, ph),
						pg.RawArgs{ph: minVal},
					))
				}
				if maxVal != "" {
					ph := fmt.Sprintf("#%s_max", colName)
					rangeConds = append(rangeConds, pg.RawBool(
						fmt.Sprintf(`%s."%s" <= %s`, tblRef, colName, ph),
						pg.RawArgs{ph: maxVal},
					))
				}
				if len(rangeConds) > 0 {
					conditions = append(conditions, pg.AND(rangeConds...))
				}
			} else {
				// 5. Default: equality
				conditions = append(conditions,
					pg.RawBool(
						fmt.Sprintf(`%s."%s" = #%s_val`, tblRef, colName, colName),
						pg.RawArgs{fmt.Sprintf("#%s_val", colName): value},
					),
				)
			}
		}
	}

	// Build SELECT statement
	stmt := pg.SELECT(projections[0], projections[1:]...).FROM(from)
	if len(conditions) > 0 {
		stmt = stmt.WHERE(pg.AND(conditions...))
	}
	groupBy := groupByPrimaryAndJoinedFields(primaryKey, moduleFields, tableRef, sortColumn(sort))
	stmt = stmt.GROUP_BY(groupBy...)
	if sort != nil {
		if sort.Direction == actions.SortDESC {
			stmt = stmt.ORDER_BY(sort.Column.DESC())
		} else {
			stmt = stmt.ORDER_BY(sort.Column.ASC())
		}
	}
	stmt = stmt.LIMIT(size).OFFSET(size * page)

	// Build COUNT statement
	countProjection := pg.COUNT(pg.STAR)
	if len(joins) > 0 {
		countProjection = pg.COUNT(pg.DISTINCT(primaryKey))
	}
	countStmt := pg.SELECT(countProjection).FROM(from)
	if len(conditions) > 0 {
		countStmt = countStmt.WHERE(pg.AND(conditions...))
	}

	query, args := stmt.Sql()
	countQuery, countArgs := countStmt.Sql()

	db.debugLog(log, "[DEBUG] LIST QUERY: ", interpolateQuery(query, args))
	db.debugLog(log, "[DEBUG] LIST COUNT QUERY: ", interpolateQuery(countQuery, countArgs))

	// Execute main query
	var rows *sql.Rows
	if len(args) > 0 {
		rows, err = db.sql.QueryContext(db.queryContext(), query, args...)
	} else {
		rows, err = db.sql.QueryContext(db.queryContext(), query)
	}
	if err != nil {
		log.Errorln("LIST ERR: ", err)
		return nil, 0, err
	}
	defer rows.Close()

	results := make([]interface{}, 0, 10)
	for rows.Next() {
		columnValues := make([]interface{}, 0, 10)
		var primaryValue interface{}
		columnValues = append(columnValues, &primaryValue)

		for i, meta := range metas {
			if meta.translatable {
				columnValues = append(columnValues, &sql.NullString{})
			} else {
				columnValues = append(columnValues, moduleFields[i].NewScanValue())
			}
		}
		for _, join := range joins {
			if len(join.Columns) == 0 {
				continue
			}
			var columnValue json.RawMessage
			columnValues = append(columnValues, &columnValue)
		}

		err = rows.Scan(columnValues...)
		if err != nil {
			log.Errorln("[DEBUG] SCAN ERR: ", err)
			return nil, 0, err
		}

		currentResult := make(map[string]interface{})
		offset := 1
		for index, meta := range metas {
			if meta.translatable {
				raw, ok := columnValues[index+offset].(*sql.NullString)
				if ok && raw.Valid {
					var langMap map[string]string
					if jsonErr := json.Unmarshal([]byte(raw.String), &langMap); jsonErr == nil {
						currentResult[meta.name] = langMap
					} else {
						currentResult[meta.name] = map[string]string{}
					}
				} else {
					currentResult[meta.name] = map[string]string{}
				}
			} else {
				field := moduleFields[index]
				currentResult[meta.name] = moduleFieldResultValue(field, columnValues[index+offset])
			}
		}

		if len(moduleFields) > 0 {
			offset = offset + len(moduleFields)
		}

		joinOffset := offset
		for _, join := range joins {
			if len(join.Columns) == 0 {
				continue
			}
			joinValue := columnValues[joinOffset]
			joinOffset++
			converted, ok := joinValue.(*json.RawMessage)
			if !ok {
				continue
			}

			var joinValues [][]interface{}
			err := json.Unmarshal(*converted, &joinValues)
			if err != nil {
				log.Errorln("LIST JOIN ERR: ", err)
				continue
			}

			checkString := ""
			for _, val := range joinValues {
				if val == nil {
					continue
				}
				for _, v := range val {
					if v == nil {
						continue
					}
					checkString = fmt.Sprintf("%v%v", checkString, v)
				}
			}

			joinResults := make([]map[string]interface{}, 0, 10)
			if len(checkString) > 0 {
				for _, joinValue := range joinValues {
					resultMap := make(map[string]interface{})
					for idx, col := range join.Columns {
						resultMap[col.Name()] = joinValue[idx]
					}
					joinResults = append(joinResults, resultMap)
				}
			}

			joinStringsArray := make([]string, 0, 10)
			for _, res := range joinResults {
				jsonRes, err := json.Marshal(res)
				if err != nil {
					continue
				}
				joinStringsArray = append(joinStringsArray, string(jsonRes))
			}
			resultUnique := removeDuplicate(joinStringsArray)

			joinResultUnique := make([]map[string]interface{}, 0, 10)
			for _, res := range resultUnique {
				var mapResult map[string]interface{}
				err := json.Unmarshal([]byte(res), &mapResult)
				if err != nil {
					continue
				}
				joinResultUnique = append(joinResultUnique, mapResult)
			}

			currentResult[join.ResultArrayName] = joinResultUnique
		}

		results = append(results, currentResult)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()

	result = append(result, results...)

	// Execute count query
	var countResult *sql.Rows
	if len(countArgs) > 0 {
		countResult, err = db.sql.QueryContext(db.queryContext(), countQuery, countArgs...)
	} else {
		countResult, err = db.sql.QueryContext(db.queryContext(), countQuery)
	}
	if err != nil {
		log.Errorln("COUNT ERR: ", err)
		return nil, 0, err
	}
	defer countResult.Close()

	var count int64
	for countResult.Next() {
		var currentCount int64
		err = countResult.Scan(&currentCount)
		if err != nil {
			return nil, 0, err
		}
		count += currentCount
	}

	if err := countResult.Err(); err != nil {
		return nil, 0, err
	}
	return result, count, nil
}

func columnTableRef(col pg.Column, fallback string) string {
	if tableName := col.TableName(); tableName != "" {
		return tableName
	}
	return fallback
}

func sortColumn(sort *actions.SortOption) pg.Column {
	if sort == nil {
		return nil
	}
	return sort.Column
}

// groupByPrimaryAndJoinedFields preserves PostgreSQL's primary-key functional
// dependency for the module table while adding selected or sorted columns from
// joined tables. Direct joined fields are valid typed module fields and must
// not be silently replaced with a base-table column in GROUP BY or ORDER BY.
func groupByPrimaryAndJoinedFields(primaryKey pg.Column, moduleFields []fields.ModuleField, baseTable string, extra ...pg.Column) []pg.GroupByClause {
	result := []pg.GroupByClause{primaryKey}
	seen := map[string]struct{}{primaryKey.TableName() + "." + primaryKey.Name(): {}}
	appendColumn := func(column pg.Column) {
		if column == nil || columnTableRef(column, baseTable) == baseTable {
			return
		}
		key := column.TableName() + "." + column.Name()
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		result = append(result, column)
	}
	for _, field := range moduleFields {
		if !field.Translatable {
			appendColumn(field.Column)
		}
	}
	for _, column := range extra {
		appendColumn(column)
	}
	return result
}

func (db *DB) View(
	log *log.Entry,
	table pg.Table,
	primaryKey pg.Column,
	moduleFields []fields.ModuleField,
	where pg.BoolExpression,
	joins []actions.ModuleActionJoin,
	tc *TranslationContext,
) (interface{}, error) {

	tableRef := table.TableName()
	pkName := primaryKey.Name()

	projections := []pg.Projection{primaryKey}
	metas := make([]fieldMeta, len(moduleFields))

	for i, field := range moduleFields {
		if field.Translatable && tc != nil {
			projections = append(projections, buildTranslationSubquery(tc, tableRef, pkName, field.Name()))
			metas[i] = fieldMeta{translatable: true, name: field.Name()}
		} else {
			projections = append(projections, field.GetProjection())
			metas[i] = fieldMeta{translatable: false, name: field.ColumnName()}
		}
	}
	for _, join := range joins {
		if len(join.Columns) > 0 {
			projections = append(projections, buildJoinProjection(join))
		}
	}

	var from pg.ReadableTable = table
	for _, join := range joins {
		from = applyJoin(from, join)
	}

	stmt := pg.SELECT(projections[0], projections[1:]...).FROM(from)
	if where != nil {
		stmt = stmt.WHERE(where)
	}
	stmt = stmt.GROUP_BY(groupByPrimaryAndJoinedFields(primaryKey, moduleFields, tableRef)...).LIMIT(1)

	query, args := stmt.Sql()
	db.debugLog(log, "[DEBUG] VIEW QUERY: ", interpolateQuery(query, args))

	rows, err := db.queryView(query, args...)
	if err != nil {
		log.Errorln("VIEW ERR: ", err)
		return nil, err
	}
	defer rows.Close()

	results := make([]interface{}, 0, 10)
	for rows.Next() {
		columnValues := make([]interface{}, 0, 10)
		var primaryValue interface{}
		columnValues = append(columnValues, &primaryValue)

		for i, meta := range metas {
			if meta.translatable {
				columnValues = append(columnValues, &sql.NullString{})
			} else {
				columnValues = append(columnValues, moduleFields[i].NewScanValue())
			}
		}
		for _, join := range joins {
			if len(join.Columns) == 0 {
				continue
			}
			var columnValue json.RawMessage
			columnValues = append(columnValues, &columnValue)
		}

		err = rows.Scan(columnValues...)
		if err != nil {
			log.Errorln("[DEBUG] VIEW SCAN ERR: ", err)
			return nil, err
		}

		currentResult := make(map[string]interface{})
		offset := 1
		for index, meta := range metas {
			if meta.translatable {
				raw, ok := columnValues[index+offset].(*sql.NullString)
				if ok && raw.Valid {
					var langMap map[string]string
					if jsonErr := json.Unmarshal([]byte(raw.String), &langMap); jsonErr == nil {
						currentResult[meta.name] = langMap
					} else {
						currentResult[meta.name] = map[string]string{}
					}
				} else {
					currentResult[meta.name] = map[string]string{}
				}
			} else {
				field := moduleFields[index]
				currentResult[meta.name] = moduleFieldResultValue(field, columnValues[index+offset])
			}
		}

		if len(moduleFields) > 0 {
			offset = offset + len(moduleFields)
		}

		joinOffset := offset
		for _, join := range joins {
			if len(join.Columns) == 0 {
				continue
			}
			joinValue := columnValues[joinOffset]
			joinOffset++
			converted, ok := joinValue.(*json.RawMessage)
			if !ok {
				continue
			}

			var joinValues [][]interface{}
			err := json.Unmarshal(*converted, &joinValues)
			if err != nil {
				log.Errorln("VIEW JOIN ERR: ", err)
				continue
			}

			checkString := ""
			for _, val := range joinValues {
				if val == nil {
					continue
				}
				for _, v := range val {
					if v == nil {
						continue
					}
					checkString = fmt.Sprintf("%v%v", checkString, v)
				}
			}

			joinResults := make([]map[string]interface{}, 0, 10)
			if len(checkString) > 0 {
				for _, joinValue := range joinValues {
					resultMap := make(map[string]interface{})
					for idx, col := range join.Columns {
						resultMap[col.Name()] = joinValue[idx]
					}
					joinResults = append(joinResults, resultMap)
				}
			}

			currentResult[join.ResultArrayName] = joinResults
			offset += 1
		}

		results = append(results, currentResult)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(results) > 0 {
		return results[0], nil
	}

	return nil, errors.New("Record not found")
}

func (db *DB) Add(log *log.Entry, table pg.Table, primaryKey pg.Column, moduleFields []fields.ModuleField, input map[string]interface{}, tc *TranslationContext) (interface{}, error) {
	tx, err := db.sql.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	output := struct {
		Value      int64  `json:"value"`
		PrimaryKey string `json:"primary_key"`
	}{}

	keys := make([]string, 0, len(input))
	values := make([]interface{}, 0, len(input))

	for _, field := range moduleFields {
		if field.Translatable {
			continue // translatable fields go into translations table
		}
		colName := field.ColumnName()
		value, ok := input[colName]
		if !ok {
			continue
		}
		keys = append(keys, fmt.Sprintf(`"%s"`, colName))
		values = append(values, dbValue(field, value))
	}

	tableName := table.TableName()
	schemaName := table.SchemaName()
	fullTableName := fmt.Sprintf(`"%s"`, tableName)
	if schemaName != "" {
		fullTableName = fmt.Sprintf(`%s."%s"`, schemaName, tableName)
	}

	if len(keys) > 0 {
		valueNumbers := make([]string, 0, len(values))
		for i := range values {
			valueNumbers = append(valueNumbers, fmt.Sprintf(`$%d`, i+1))
		}

		query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s) RETURNING "%s"`,
			fullTableName,
			strings.Join(keys, ","),
			strings.Join(valueNumbers, ","),
			primaryKey.Name(),
		)

		db.debugLog(log, "[DEBUG] ADD QUERY: ", interpolateQuery(query, values))

		err = tx.QueryRow(query, values...).Scan(&output.Value)
		if err != nil {
			log.Errorln("ADD ERR: ", err)
			return nil, err
		}
	} else {
		// All fields are translatable — insert default row
		query := fmt.Sprintf(`INSERT INTO %s DEFAULT VALUES RETURNING "%s"`,
			fullTableName, primaryKey.Name(),
		)
		db.debugLog(log, "[DEBUG] ADD QUERY (default): ", query)
		err = tx.QueryRow(query).Scan(&output.Value)
		if err != nil {
			log.Errorln("ADD ERR: ", err)
			return nil, err
		}
	}

	output.PrimaryKey = primaryKey.Name()

	// Insert translations
	if tc != nil {
		if err = InsertTranslations(tx, tc, output.Value, moduleFields, input); err != nil {
			log.Errorln("ADD TRANSLATIONS ERR: ", err)
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return output, nil
}

func (db *DB) Update(log *log.Entry, table pg.Table, primaryKey pg.Column, moduleFields []fields.ModuleField, input map[string]interface{}, where pg.BoolExpression, tc *TranslationContext) (interface{}, error) {
	tx, err := db.sql.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if where == nil {
		return nil, errors.New("WHERE condition is required for UPDATE")
	}

	setClauses := make([]string, 0, len(input))
	values := make([]interface{}, 0, len(input))
	paramIdx := 1

	for _, field := range moduleFields {
		if field.Translatable {
			continue // handled separately
		}
		colName := field.ColumnName()
		value, ok := input[colName]
		if !ok {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf(`"%s" = $%d`, colName, paramIdx))
		values = append(values, dbValue(field, value))
		paramIdx++
	}

	// Only run UPDATE on entity table if there are non-translatable fields to update
	if len(setClauses) > 0 {
		if hasModuleField(moduleFields, "update_date") {
			setClauses = append(setClauses, fmt.Sprintf(`"update_date" = $%d`, paramIdx))
			values = append(values, time.Now())
			paramIdx++
		}

		// Get WHERE clause SQL from a dummy select
		whereStmt := pg.SELECT(pg.Raw("1")).FROM(table).WHERE(where)
		whereSql, whereArgs := whereStmt.Sql()

		// Extract WHERE part from the full query
		whereIdx := strings.Index(whereSql, "WHERE")
		if whereIdx == -1 {
			return nil, errors.New("could not build WHERE clause")
		}
		whereClause := strings.TrimRight(whereSql[whereIdx:], ";\n\r\t ")

		whereClause = renumberPlaceholders(whereClause, paramIdx)
		values = append(values, whereArgs...)
		paramIdx += len(whereArgs)

		tableName := table.TableName()
		schemaName := table.SchemaName()
		fullTableName := fmt.Sprintf(`"%s"`, tableName)
		if schemaName != "" {
			fullTableName = fmt.Sprintf(`%s."%s"`, schemaName, tableName)
		}

		query := fmt.Sprintf(`UPDATE %s SET %s %s`,
			fullTableName,
			strings.Join(setClauses, ", "),
			whereClause,
		)

		db.debugLog(log, "[DEBUG] UPDATE QUERY: ", interpolateQuery(query, values))

		result, err := tx.Exec(query, values...)
		if err != nil {
			return nil, err
		}

		updatedCount, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}

		if updatedCount == 0 {
			return nil, errors.New("record not found")
		}
	}

	// Upsert translations
	if tc != nil && tc.EntityID != nil {
		if err = UpsertTranslations(tx, tc, tc.EntityID, moduleFields, input); err != nil {
			log.Errorln("UPDATE TRANSLATIONS ERR: ", err)
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return db.View(log, table, primaryKey, moduleFields, where, nil, tc)
}

func hasModuleField(moduleFields []fields.ModuleField, columnName string) bool {
	for _, field := range moduleFields {
		if field.ColumnName() == columnName {
			return true
		}
	}
	return false
}

func dbValue(field fields.ModuleField, value interface{}) interface{} {
	switch field.Type {
	case fields.ModuleFieldTypeObject:
		encoded, err := fields.MarshalJSONObject(value)
		if err != nil {
			return value
		}
		return string(encoded)
	case fields.ModuleFieldTypeArray:
		if field.ArrayStorage.Normalize() == fields.ModuleFieldArrayStorageJSON {
			encoded, err := fields.MarshalJSONArray(value)
			if err == nil {
				return string(encoded)
			}
			return value
		}
	default:
		return value
	}

	switch typed := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, fmt.Sprint(item))
		}
		return pq.Array(result)
	case []string:
		return pq.Array(typed)
	default:
		return value
	}
}

var placeholderPattern = regexp.MustCompile(`\$(\d+)`)

func renumberPlaceholders(query string, start int) string {
	return placeholderPattern.ReplaceAllStringFunc(query, func(match string) string {
		index, err := strconv.Atoi(strings.TrimPrefix(match, "$"))
		if err != nil || index <= 0 {
			return match
		}
		return fmt.Sprintf("$%d", start+index-1)
	})
}

func (db *DB) Delete(log *log.Entry, table pg.Table, where pg.BoolExpression, tc *TranslationContext) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete translations first
	if tc != nil && tc.EntityID != nil {
		_, err = tx.Exec(
			`DELETE FROM translations WHERE entity = $1 AND entity_id = $2`,
			tc.EntityName, tc.EntityID,
		)
		if err != nil {
			return err
		}
	}

	stmt := table.DELETE().WHERE(where)
	query, args := stmt.Sql()

	db.debugLog(log, "[DEBUG] DELETE QUERY: ", interpolateQuery(query, args))

	result, err := tx.Exec(query, args...)
	if err != nil {
		return err
	}

	countOfDeleted, err := result.RowsAffected()
	if err != nil {
		return err
	}

	db.debugLog(log, "[DEBUG] DELETE COUNT OF DELETED: ", countOfDeleted)
	if countOfDeleted == 0 {
		return errors.New("record not found")
	}

	return tx.Commit()
}

func (db *DB) RawRequest(log *log.Entry, query string, params ...interface{}) (*sql.Rows, error) {
	return db.sql.Query(query, params...)
}

func (db *DB) RawDB() *sql.DB {
	return db.sql
}

func removeDuplicate(sliceList []string) []string {
	allKeys := make(map[string]bool)
	var list []string
	for _, item := range sliceList {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}
