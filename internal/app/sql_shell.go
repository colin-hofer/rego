package app

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func runSQLShell(ctx context.Context, connection *sql.DB) error {
	if connection == nil {
		return fmt.Errorf("database connection not available")
	}

	reader := bufio.NewReader(os.Stdin)
	var statement strings.Builder

	for {
		prompt := "rego=> "
		if statement.Len() > 0 {
			prompt = "....=> "
		}
		if _, err := fmt.Fprint(os.Stdout, prompt); err != nil {
			return fmt.Errorf("write shell prompt: %w", err)
		}

		line, err := reader.ReadString('\n')
		eof := errors.Is(err, io.EOF)
		if err != nil && !eof {
			return fmt.Errorf("read shell input: %w", err)
		}

		trimmed := strings.TrimSpace(line)
		if statement.Len() == 0 {
			switch trimmed {
			case "\\q", "quit", "exit":
				return nil
			case "":
				if eof {
					return nil
				}
				continue
			}
		}

		statement.WriteString(line)
		if !eof && !isStatementComplete(statement.String()) {
			continue
		}

		sqlStatement := strings.TrimSpace(statement.String())
		statement.Reset()
		if sqlStatement == "" {
			if eof {
				return nil
			}
			continue
		}

		execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = executeSQL(execCtx, connection, sqlStatement)
		cancel()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}

		if eof {
			return nil
		}
	}
}

func isStatementComplete(statement string) bool {
	return strings.HasSuffix(strings.TrimSpace(statement), ";")
}

func executeSQL(ctx context.Context, connection *sql.DB, statement string) error {
	statement = strings.TrimSpace(statement)
	statement = strings.TrimSuffix(statement, ";")
	if statement == "" {
		return nil
	}

	if shouldQuery(statement) {
		return queryAndPrint(ctx, connection, statement)
	}

	result, err := connection.ExecContext(ctx, statement)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		fmt.Fprintln(os.Stdout, "OK")
		return nil
	}

	label := "rows"
	if rowsAffected == 1 {
		label = "row"
	}
	fmt.Fprintf(os.Stdout, "OK (%d %s affected)\n", rowsAffected, label)
	return nil
}

func shouldQuery(statement string) bool {
	statement = strings.TrimSpace(strings.ToLower(statement))
	prefixes := []string{"select", "with", "show", "values", "table", "explain"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(statement, prefix+" ") || statement == prefix {
			return true
		}
	}

	return false
}

func queryAndPrint(ctx context.Context, connection *sql.DB, statement string) error {
	rows, err := connection.QueryContext(ctx, statement)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	widths := make([]int, len(columns))
	for i, column := range columns {
		widths[i] = len(column)
	}

	records := make([][]string, 0)
	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return err
		}

		record := make([]string, len(values))
		for i, value := range values {
			rendered := renderValue(value)
			record[i] = rendered
			if len(rendered) > widths[i] {
				widths[i] = len(rendered)
			}
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	printTable(columns, widths, records)
	return nil
}

func renderValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(typed)
	}
}

func printTable(columns []string, widths []int, records [][]string) {
	printTableRow(columns, widths)
	printTableDivider(widths)
	for _, record := range records {
		printTableRow(record, widths)
	}

	label := "rows"
	if len(records) == 1 {
		label = "row"
	}
	fmt.Fprintf(os.Stdout, "(%d %s)\n", len(records), label)
}

func printTableRow(values []string, widths []int) {
	for i, value := range values {
		if i > 0 {
			fmt.Fprint(os.Stdout, " | ")
		}
		fmt.Fprintf(os.Stdout, "%-*s", widths[i], value)
	}
	fmt.Fprintln(os.Stdout)
}

func printTableDivider(widths []int) {
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(os.Stdout, "-+-")
		}
		fmt.Fprint(os.Stdout, strings.Repeat("-", width))
	}
	fmt.Fprintln(os.Stdout)
}
