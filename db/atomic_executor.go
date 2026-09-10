package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/darkrain/request-generator/actions"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/lib/pq"
)

type atomicExecutor struct {
	tx *sql.Tx
}

// NewAtomicExecutor adapts a generator-owned SQL transaction to the public
// atomic action contract without exposing the transaction to modules.
func NewAtomicExecutor(tx *sql.Tx) actions.AtomicExecutor {
	return atomicExecutor{tx: tx}
}

func (executor atomicExecutor) Insert(ctx context.Context, insert actions.AtomicInsert) (actions.AtomicRecord, error) {
	if insert.Table == nil || insert.PrimaryKey == nil {
		return actions.AtomicRecord{}, fmt.Errorf("atomic insert requires table and primary key")
	}
	if len(insert.Fields) == 0 {
		return actions.AtomicRecord{}, fmt.Errorf("atomic insert requires fields")
	}

	keys := make([]string, 0, len(insert.Fields))
	placeholders := make([]string, 0, len(insert.Fields))
	values := make([]interface{}, 0, len(insert.Fields))
	for index, field := range insert.Fields {
		if field.Column == nil {
			return actions.AtomicRecord{}, fmt.Errorf("atomic insert field %d has no column", index)
		}
		if err := field.Value.Validate(); err != nil {
			return actions.AtomicRecord{}, fmt.Errorf("atomic insert field %q: %w", field.Column.Name(), err)
		}
		keys = append(keys, fmt.Sprintf(`"%s"`, field.Column.Name()))
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
		values = append(values, atomicDBValue(field.Value))
	}

	tableName := fmt.Sprintf(`"%s"`, insert.Table.TableName())
	if schema := insert.Table.SchemaName(); schema != "" {
		tableName = fmt.Sprintf(`%s."%s"`, schema, insert.Table.TableName())
	}
	query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s) RETURNING "%s"`, tableName, strings.Join(keys, ","), strings.Join(placeholders, ","), insert.PrimaryKey.Name())

	var value int64
	if err := executor.tx.QueryRowContext(ctx, query, values...).Scan(&value); err != nil {
		return actions.AtomicRecord{}, err
	}
	return actions.AtomicRecord{Value: value, PrimaryKey: insert.PrimaryKey.Name()}, nil
}

func (executor atomicExecutor) Upsert(ctx context.Context, upsert actions.AtomicUpsert) (actions.AtomicUpsertResult, error) {
	insert := upsert.Insert
	if insert.Table == nil || insert.PrimaryKey == nil {
		return actions.AtomicUpsertResult{}, fmt.Errorf("atomic upsert requires table and primary key")
	}
	if len(insert.Fields) == 0 {
		return actions.AtomicUpsertResult{}, fmt.Errorf("atomic upsert requires insert fields")
	}
	if len(upsert.ConflictColumns) == 0 {
		return actions.AtomicUpsertResult{}, fmt.Errorf("atomic upsert requires conflict columns")
	}

	keys := make([]string, 0, len(insert.Fields))
	placeholders := make([]string, 0, len(insert.Fields))
	values := make([]interface{}, 0, len(insert.Fields)+len(upsert.UpdateFields))
	for index, field := range insert.Fields {
		if field.Column == nil {
			return actions.AtomicUpsertResult{}, fmt.Errorf("atomic upsert insert field %d has no column", index)
		}
		if err := field.Value.Validate(); err != nil {
			return actions.AtomicUpsertResult{}, fmt.Errorf("atomic upsert insert field %q: %w", field.Column.Name(), err)
		}
		keys = append(keys, fmt.Sprintf(`"%s"`, field.Column.Name()))
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
		values = append(values, atomicDBValue(field.Value))
	}

	conflicts := make([]string, 0, len(upsert.ConflictColumns))
	for index, column := range upsert.ConflictColumns {
		if column == nil {
			return actions.AtomicUpsertResult{}, fmt.Errorf("atomic upsert conflict column %d is nil", index)
		}
		conflicts = append(conflicts, fmt.Sprintf(`"%s"`, column.Name()))
	}

	resolution := "DO NOTHING"
	if len(upsert.UpdateFields) > 0 {
		assignments := make([]string, 0, len(upsert.UpdateFields))
		for index, field := range upsert.UpdateFields {
			if field.Column == nil {
				return actions.AtomicUpsertResult{}, fmt.Errorf("atomic upsert update field %d has no column", index)
			}
			if err := field.Value.Validate(); err != nil {
				return actions.AtomicUpsertResult{}, fmt.Errorf("atomic upsert update field %q: %w", field.Column.Name(), err)
			}
			values = append(values, atomicDBValue(field.Value))
			assignments = append(assignments, fmt.Sprintf(`"%s" = $%d`, field.Column.Name(), len(values)))
		}
		resolution = "DO UPDATE SET " + strings.Join(assignments, ",")
	}

	tableName := fmt.Sprintf(`"%s"`, insert.Table.TableName())
	if schema := insert.Table.SchemaName(); schema != "" {
		tableName = fmt.Sprintf(`%s."%s"`, schema, insert.Table.TableName())
	}
	query := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) %s RETURNING "%s", (xmax = 0) AS inserted`,
		tableName,
		strings.Join(keys, ","),
		strings.Join(placeholders, ","),
		strings.Join(conflicts, ","),
		resolution,
		insert.PrimaryKey.Name(),
	)

	var value int64
	var inserted bool
	err := executor.tx.QueryRowContext(ctx, query, values...).Scan(&value, &inserted)
	if err == sql.ErrNoRows {
		return actions.AtomicUpsertResult{Inserted: false}, nil
	}
	if err != nil {
		return actions.AtomicUpsertResult{}, err
	}
	return actions.AtomicUpsertResult{
		Record:   actions.AtomicRecord{Value: value, PrimaryKey: insert.PrimaryKey.Name()},
		Inserted: inserted,
	}, nil
}

func (executor atomicExecutor) SelectOne(ctx context.Context, selectRequest actions.AtomicSelect) (actions.AtomicRecord, error) {
	if err := validateAtomicSelect(selectRequest); err != nil {
		return actions.AtomicRecord{}, err
	}
	rows, err := executor.selectRows(ctx, selectRequest, nil, 1)
	if err != nil {
		return actions.AtomicRecord{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return actions.AtomicRecord{}, err
		}
		return actions.AtomicRecord{}, sql.ErrNoRows
	}
	record, err := scanAtomicRecord(rows, selectRequest.Fields)
	if err != nil {
		return actions.AtomicRecord{}, err
	}
	if err := rows.Err(); err != nil {
		return actions.AtomicRecord{}, err
	}
	return record, nil
}

func (executor atomicExecutor) SelectMany(ctx context.Context, selectRequest actions.AtomicSelectMany) ([]actions.AtomicRecord, error) {
	if err := validateAtomicSelect(selectRequest.AtomicSelect); err != nil {
		return nil, err
	}
	if len(selectRequest.OrderBy) == 0 {
		return nil, fmt.Errorf("atomic select many requires order by")
	}
	if selectRequest.Limit <= 0 {
		return nil, fmt.Errorf("atomic select many requires a positive limit")
	}
	rows, err := executor.selectRows(ctx, selectRequest.AtomicSelect, selectRequest.OrderBy, selectRequest.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]actions.AtomicRecord, 0)
	for rows.Next() {
		record, err := scanAtomicRecord(rows, selectRequest.Fields)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (executor atomicExecutor) Update(ctx context.Context, update actions.AtomicUpdate) (int64, error) {
	if update.Table == nil || len(update.Fields) == 0 || update.Where == nil {
		return 0, fmt.Errorf("atomic update requires table, fields, and where")
	}
	assignments := make([]pg.ColumnAssigment, 0, len(update.Fields))
	for index, field := range update.Fields {
		assignment, err := atomicUpdateAssignment(field)
		if err != nil {
			return 0, fmt.Errorf("atomic update field %d: %w", index, err)
		}
		assignments = append(assignments, assignment)
	}
	remaining := make([]interface{}, 0, len(assignments)-1)
	for _, assignment := range assignments[1:] {
		remaining = append(remaining, assignment)
	}
	statement, args := update.Table.UPDATE().SET(assignments[0], remaining...).WHERE(update.Where).Sql()
	result, err := executor.tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func atomicUpdateAssignment(field actions.AtomicUpdateField) (pg.ColumnAssigment, error) {
	if field.Column == nil {
		return nil, fmt.Errorf("column is required")
	}
	if field.Operation == actions.AtomicUpdateClear {
		switch column := field.Column.(type) {
		case pg.ColumnTimestampz:
			return column.SET(pg.TimestampzExp(pg.NULL)), nil
		case pg.ColumnTimestamp:
			return column.SET(pg.TimestampExp(pg.NULL)), nil
		case pg.ColumnDate:
			return column.SET(pg.DateExp(pg.NULL)), nil
		case pg.ColumnString:
			return column.SET(pg.StringExp(pg.NULL)), nil
		case pg.ColumnInteger:
			return column.SET(pg.IntExp(pg.NULL)), nil
		case pg.ColumnFloat:
			return column.SET(pg.FloatExp(pg.NULL)), nil
		default:
			return nil, fmt.Errorf("column %q cannot be cleared", field.Column.Name())
		}
	}
	if err := field.Value.Validate(); err != nil {
		return nil, err
	}
	switch column := field.Column.(type) {
	case pg.ColumnInteger:
		if field.Value.Int == nil {
			return nil, fmt.Errorf("integer column %q requires int value", column.Name())
		}
		switch field.Operation {
		case actions.AtomicUpdateSet:
			return column.SET(pg.Int(*field.Value.Int)), nil
		case actions.AtomicUpdateIncrement:
			return column.SET(column.ADD(pg.Int(*field.Value.Int))), nil
		}
	case pg.ColumnFloat:
		if field.Value.Float == nil {
			return nil, fmt.Errorf("float column %q requires float value", column.Name())
		}
		switch field.Operation {
		case actions.AtomicUpdateSet:
			return column.SET(pg.Float(*field.Value.Float)), nil
		case actions.AtomicUpdateIncrement:
			return column.SET(column.ADD(pg.Float(*field.Value.Float))), nil
		}
	case pg.ColumnString:
		if field.Value.String == nil {
			return nil, fmt.Errorf("string column %q requires string value", column.Name())
		}
		if field.Operation == actions.AtomicUpdateSet {
			return column.SET(pg.String(*field.Value.String)), nil
		}
	case pg.ColumnBool:
		if field.Value.Bool == nil {
			return nil, fmt.Errorf("bool column %q requires bool value", column.Name())
		}
		if field.Operation == actions.AtomicUpdateSet {
			return column.SET(pg.Bool(*field.Value.Bool)), nil
		}
	case pg.ColumnTimestamp:
		if field.Value.Time == nil {
			return nil, fmt.Errorf("timestamp column %q requires time value", column.Name())
		}
		if field.Operation == actions.AtomicUpdateSet {
			return column.SET(pg.TimestampT(*field.Value.Time)), nil
		}
	case pg.ColumnTimestampz:
		if field.Value.Time == nil {
			return nil, fmt.Errorf("timestampz column %q requires time value", column.Name())
		}
		if field.Operation == actions.AtomicUpdateSet {
			return column.SET(pg.TimestampzT(*field.Value.Time)), nil
		}
	case pg.ColumnDate:
		if field.Value.Time == nil {
			return nil, fmt.Errorf("date column %q requires time value", column.Name())
		}
		if field.Operation == actions.AtomicUpdateSet {
			return column.SET(pg.DateT(*field.Value.Time)), nil
		}
	}
	return nil, fmt.Errorf("operation %q is unsupported for column %q", field.Operation, field.Column.Name())
}

func validateAtomicSelect(selectRequest actions.AtomicSelect) error {
	if selectRequest.Table == nil || len(selectRequest.Fields) == 0 || selectRequest.Where == nil {
		return fmt.Errorf("atomic select requires table, fields, and where")
	}
	return nil
}

func (executor atomicExecutor) selectRows(ctx context.Context, selectRequest actions.AtomicSelect, orderBy []pg.OrderByClause, limit int) (*sql.Rows, error) {
	projections := make([]pg.Projection, 0, len(selectRequest.Fields))
	for _, field := range selectRequest.Fields {
		if field.Name == "" || field.Column == nil {
			return nil, fmt.Errorf("atomic select field requires name and column")
		}
		if _, _, err := atomicSelectScan(field.Kind); err != nil {
			return nil, fmt.Errorf("atomic select field %q: %w", field.Name, err)
		}
		projections = append(projections, field.Column)
	}
	query := pg.SELECT(projections[0], projections[1:]...).FROM(selectRequest.Table).WHERE(selectRequest.Where)
	if len(orderBy) > 0 {
		query = query.ORDER_BY(orderBy...)
	}
	query = query.LIMIT(int64(limit))
	statement, args := query.Sql()
	return executor.tx.QueryContext(ctx, statement, args...)
}

func scanAtomicRecord(rows *sql.Rows, fields []actions.AtomicSelectField) (actions.AtomicRecord, error) {
	scans := make([]interface{}, 0, len(fields))
	values := make([]func() (actions.AtomicValue, error), 0, len(fields))
	for _, field := range fields {
		scan, value, err := atomicSelectScan(field.Kind)
		if err != nil {
			return actions.AtomicRecord{}, fmt.Errorf("atomic select field %q: %w", field.Name, err)
		}
		scans = append(scans, scan)
		values = append(values, value)
	}
	if err := rows.Scan(scans...); err != nil {
		return actions.AtomicRecord{}, err
	}
	record := actions.AtomicRecord{Fields: make([]actions.AtomicField, 0, len(fields))}
	for index, field := range fields {
		value, err := values[index]()
		if err != nil {
			return actions.AtomicRecord{}, fmt.Errorf("atomic select field %q: %w", field.Name, err)
		}
		record.Fields = append(record.Fields, actions.AtomicField{Name: field.Name, Value: value})
	}
	return record, nil
}

func atomicSelectScan(kind actions.AtomicValueKind) (interface{}, func() (actions.AtomicValue, error), error) {
	switch kind {
	case actions.AtomicValueKindString:
		value := &sql.NullString{}
		return value, func() (actions.AtomicValue, error) {
			if !value.Valid {
				return actions.AtomicValue{}, fmt.Errorf("value is null")
			}
			return actions.AtomicString(value.String), nil
		}, nil
	case actions.AtomicValueKindNullableString:
		value := &sql.NullString{}
		return value, func() (actions.AtomicValue, error) {
			if !value.Valid {
				return actions.AtomicString(""), nil
			}
			return actions.AtomicString(value.String), nil
		}, nil
	case actions.AtomicValueKindInt:
		value := &sql.NullInt64{}
		return value, func() (actions.AtomicValue, error) {
			if !value.Valid {
				return actions.AtomicValue{}, fmt.Errorf("value is null")
			}
			return actions.AtomicInt(value.Int64), nil
		}, nil
	case actions.AtomicValueKindNullableInt:
		// A column that may hold nothing answers "nothing" rather than failing
		// the whole read: the caller decides what an absent link means.
		value := &sql.NullInt64{}
		return value, func() (actions.AtomicValue, error) {
			if !value.Valid {
				return actions.AtomicValue{}, nil
			}
			return actions.AtomicInt(value.Int64), nil
		}, nil
	case actions.AtomicValueKindFloat:
		value := &sql.NullFloat64{}
		return value, func() (actions.AtomicValue, error) {
			if !value.Valid {
				return actions.AtomicValue{}, fmt.Errorf("value is null")
			}
			return actions.AtomicFloat(value.Float64), nil
		}, nil
	case actions.AtomicValueKindBool:
		value := &sql.NullBool{}
		return value, func() (actions.AtomicValue, error) {
			if !value.Valid {
				return actions.AtomicValue{}, fmt.Errorf("value is null")
			}
			return actions.AtomicBool(value.Bool), nil
		}, nil
	case actions.AtomicValueKindTime:
		value := &sql.NullTime{}
		return value, func() (actions.AtomicValue, error) {
			if !value.Valid {
				return actions.AtomicValue{}, fmt.Errorf("value is null")
			}
			return actions.AtomicTime(value.Time), nil
		}, nil
	case actions.AtomicValueKindInts:
		value := &pq.Int64Array{}
		return value, func() (actions.AtomicValue, error) { return actions.AtomicValue{Ints: []int64(*value)}, nil }, nil
	case actions.AtomicValueKindStrings:
		value := &pq.StringArray{}
		return value, func() (actions.AtomicValue, error) { return actions.AtomicValue{Strings: []string(*value)}, nil }, nil
	default:
		return nil, nil, fmt.Errorf("unsupported kind %q", kind)
	}
}

func atomicDBValue(value actions.AtomicValue) interface{} {
	if value.Strings != nil {
		return pq.Array(value.Strings)
	}
	if value.Ints != nil {
		return pq.Array(value.Ints)
	}
	if value.JSON != nil {
		return string(value.JSON)
	}
	return value.Interface()
}
