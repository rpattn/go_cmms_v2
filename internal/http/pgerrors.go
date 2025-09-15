package httpserver

import (
    "errors"
    "net/http"
    "strings"

    "github.com/jackc/pgx/v5/pgconn"
)

// PGErrorMessage maps common Postgres errors to user-friendly HTTP status + message.
// If err is not a pg error, returns 500 with the provided fallback message.
func PGErrorMessage(err error, fallback string) (int, string) {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) {
        // Unknown error type; hide details
        return http.StatusInternalServerError, fallback
    }

    code := pgErr.Code
    // Default mapping
    status := http.StatusBadRequest
    msg := fallback

    switch code {
    case "23505": // unique_violation
        status = http.StatusConflict
        switch pgErr.ConstraintName {
        case "app_tables_org_slug_uniq", "tables_slug_key":
            msg = "A table with this name already exists in your organisation."
        case "columns_table_name_unique":
            msg = "A column with this name already exists for this table."
        default:
            msg = "Duplicate value violates a unique constraint."
        }
    case "23503": // foreign_key_violation
        status = http.StatusBadRequest
        msg = "Referenced record not found."
    case "23514": // check_violation
        status = http.StatusBadRequest
        if pgErr.Detail != "" { msg = pgErr.Detail } else { msg = "Value violates a check constraint." }
    case "23502": // not_null_violation
        status = http.StatusBadRequest
        msg = "Missing required field."
    case "22P02": // invalid_text_representation (e.g., UUID/boolean/date)
        status = http.StatusBadRequest
        msg = "Invalid value format."
    case "22007": // invalid_datetime_format
        status = http.StatusBadRequest
        msg = "Invalid date/time format."
    case "22001": // string_data_right_truncation
        status = http.StatusBadRequest
        msg = "Value is too long."
    case "22003": // numeric_value_out_of_range
        status = http.StatusBadRequest
        msg = "Numeric value out of range."
    case "P0001": // raise_exception from functions
        status = http.StatusBadRequest
        // Surface known messages safely, otherwise use fallback
        m := pgErr.Message
        switch {
        case strings.Contains(m, "Unknown column"):
            msg = m
        case strings.Contains(m, "Required column"):
            msg = m
        case strings.Contains(m, "Missing required columns"):
            msg = m
        default:
            msg = fallback
        }
    default:
        // For any other PG error, avoid leaking internals
        status = http.StatusBadRequest
        msg = fallback
    }

    return status, msg
}

