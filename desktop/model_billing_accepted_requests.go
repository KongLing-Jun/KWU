package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

type desktopAcceptedQuotaRequestRegistry struct {
	path string
}

func defaultDesktopAcceptedQuotaRequestRegistry() desktopAcceptedQuotaRequestRegistry {
	return desktopAcceptedQuotaRequestRegistry{
		path: filepath.Join(desktopConfigDir(), "model-billing-accepted-requests.sqlite"),
	}
}

func (r desktopAcceptedQuotaRequestRegistry) AcceptQuotaRequestIDs(
	ctx context.Context,
	cloudUserID string,
	taskID string,
	requestIDs []string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(cloudUserID) != cloudUserID || cloudUserID == "" ||
		strings.TrimSpace(taskID) != taskID || taskID == "" || strings.TrimSpace(r.path) == "" {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "accepted quota request registry identity is invalid"}
	}
	unique := make(map[string]struct{}, len(requestIDs))
	for _, requestID := range requestIDs {
		if strings.TrimSpace(requestID) != requestID || requestID == "" {
			return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "accepted quota request ID is invalid"}
		}
		unique[requestID] = struct{}{}
	}
	if len(unique) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	db, err := modelbilling.OpenSQLite(r.path)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS accepted_quota_request_ids (
		cloud_user_id TEXT NOT NULL,
		request_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		PRIMARY KEY (cloud_user_id, request_id)
	)`); err != nil {
		return err
	}
	ordered := make([]string, 0, len(unique))
	for requestID := range unique {
		ordered = append(ordered, requestID)
	}
	sort.Strings(ordered)
	for _, requestID := range ordered {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO accepted_quota_request_ids
			(cloud_user_id,request_id,task_id) VALUES (?,?,?)`, cloudUserID, requestID, taskID); err != nil {
			return err
		}
		var owner string
		if err := tx.QueryRowContext(ctx, `SELECT task_id FROM accepted_quota_request_ids
			WHERE cloud_user_id=? AND request_id=?`, cloudUserID, requestID).Scan(&owner); err != nil {
			return err
		}
		if owner != taskID {
			return &modelbilling.Error{
				Code:    modelbilling.ErrorQuotaRequestIDReused,
				Message: fmt.Sprintf("oneapi request id %q was accepted for another task", requestID),
			}
		}
	}
	return tx.Commit()
}

var _ modelbilling.AcceptedQuotaRequestRegistry = desktopAcceptedQuotaRequestRegistry{}

func backfillDesktopAcceptedQuotaRequests(
	ctx context.Context,
	currentDB *sql.DB,
	scope modelbilling.TaskScope,
	registry modelbilling.AcceptedQuotaRequestRegistry,
) error {
	if currentDB == nil || scope.CloudBinding == nil || registry == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "accepted quota request backfill dependencies are required"}
	}
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}.Normalize()
	paths, err := existingDesktopBillingDatabasePaths(actor)
	if err != nil {
		return err
	}
	currentPath := filepath.Join(config.NewProfilePathResolver(actor).MemoryUserDir(), "model-billing.sqlite")
	currentScanned := false
	for _, path := range paths {
		db := currentDB
		closeDB := false
		if !desktopBillingPathsEqual(path, currentPath) {
			db, err = modelbilling.OpenSQLite(path)
			if err != nil {
				return fmt.Errorf("open accepted quota history %q: %w", path, err)
			}
			closeDB = true
		} else {
			currentScanned = true
		}
		backfillErr := modelbilling.NewSQLiteReportStore(db).BackfillAcceptedQuotaRequestIDs(
			ctx, scope.CloudBinding.CloudUserID, registry,
		)
		if closeDB {
			closeErr := db.Close()
			if backfillErr == nil {
				backfillErr = closeErr
			}
		}
		if backfillErr != nil {
			return fmt.Errorf("backfill accepted quota history %q: %w", path, backfillErr)
		}
	}
	if !currentScanned {
		if err := modelbilling.NewSQLiteReportStore(currentDB).BackfillAcceptedQuotaRequestIDs(
			ctx, scope.CloudBinding.CloudUserID, registry,
		); err != nil {
			return fmt.Errorf("backfill current accepted quota history: %w", err)
		}
	}
	return nil
}

func existingDesktopBillingDatabasePaths(actor userctx.ActorContext) ([]string, error) {
	base := strings.TrimSpace(config.NewProfilePathResolver(actor).MemoryProjectStoreRoot())
	if base == "" {
		return nil, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "desktop billing state root is required"}
	}
	var paths []string
	addExisting := func(path string) error {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode().IsRegular() {
			paths = append(paths, filepath.Clean(path))
		}
		return nil
	}
	if err := addExisting(filepath.Join(base, "model-billing.sqlite")); err != nil {
		return nil, err
	}
	profilesRoot := filepath.Join(base, "profiles")
	entries, err := os.ReadDir(profilesRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := addExisting(filepath.Join(profilesRoot, entry.Name(), "model-billing.sqlite")); err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func desktopBillingPathsEqual(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if os.PathSeparator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}
