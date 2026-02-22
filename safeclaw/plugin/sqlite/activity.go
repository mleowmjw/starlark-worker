package sqlite

import (
	"context"
	"fmt"
)

// SQLExecInput contains parameters for SQL exec operation.
type SQLExecInput struct {
	DBPath string
	SQL    string
}

// SQLExecOutput contains the result of SQL exec.
type SQLExecOutput struct {
	RowsAffected int64
	LastInsertID int64
}

// SQLQueryInput contains parameters for SQL query operation.
type SQLQueryInput struct {
	DBPath string
	SQL    string
}

// SQLQueryOutput contains the result of SQL query.
type SQLQueryOutput struct {
	Columns []string
	Rows    [][]interface{}
}

// SQLExecActivity executes a SQL statement as a Temporal activity.
func SQLExecActivity(ctx context.Context, input SQLExecInput) (*SQLExecOutput, error) {
	db, err := openDB(input.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	res, err := db.ExecContext(ctx, input.SQL)
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w", err)
	}

	rows, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()

	return &SQLExecOutput{
		RowsAffected: rows,
		LastInsertID: lastID,
	}, nil
}

// SQLQueryActivity executes a SQL query as a Temporal activity.
func SQLQueryActivity(ctx context.Context, input SQLQueryInput) (*SQLQueryOutput, error) {
	db, err := openDB(input.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, input.SQL)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns failed: %w", err)
	}

	result := [][]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		scanArgs := make([]interface{}, len(cols))
		for i := range values {
			scanArgs[i] = &values[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		result = append(result, values)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return &SQLQueryOutput{
		Columns: cols,
		Rows:    result,
	}, nil
}
