package sample

import (
	"context"
	"database/sql"
	"net/http"
)

type Reconciler struct {
	db *sql.DB
}

func (r *Reconciler) Reconcile(ctx context.Context) error {
	// ignored error (assignment to _)
	_ = r.db.Ping()

	// network call
	http.Get("http://example.com")

	// goroutine launch
	go func() {
		_, _ = r.db.Query("SELECT 1")
	}()

	// database call with error handling
	rows, err := r.db.Query("SELECT * FROM users")
	if err != nil {
		return err
	}
	defer rows.Close()

	return nil
}
