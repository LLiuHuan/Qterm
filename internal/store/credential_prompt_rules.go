package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"changeme/internal/model"
)

const credentialPromptRuleColumns = `id,kind,pattern,enabled,sort_order,created_at,updated_at`

func (s *Store) ListCredentialPromptRules(ctx context.Context) ([]model.CredentialPromptRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+credentialPromptRuleColumns+` FROM credential_prompt_rules ORDER BY sort_order,id`)
	if err != nil {
		return nil, fmt.Errorf("list credential prompt rules: %w", err)
	}
	defer rows.Close()
	rules := make([]model.CredentialPromptRule, 0)
	for rows.Next() {
		rule, err := scanCredentialPromptRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *Store) CreateCredentialPromptRule(ctx context.Context, rule model.CredentialPromptRule) (model.CredentialPromptRule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("begin credential prompt rule: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order),-1)+1 FROM credential_prompt_rules`).Scan(&rule.SortOrder); err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("next credential prompt rule order: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credential_prompt_rules(`+credentialPromptRuleColumns+`) VALUES(?,?,?,?,?,?,?)`,
		rule.ID, rule.Kind, rule.Pattern, boolInt(rule.Enabled), rule.SortOrder, formatTime(rule.CreatedAt), formatTime(rule.UpdatedAt)); err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("insert credential prompt rule: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("commit credential prompt rule: %w", err)
	}
	return rule, nil
}

func (s *Store) UpdateCredentialPromptRule(ctx context.Context, rule model.CredentialPromptRule) (model.CredentialPromptRule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("begin credential prompt rule update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	existing, err := scanCredentialPromptRule(tx.QueryRowContext(ctx, `SELECT `+credentialPromptRuleColumns+` FROM credential_prompt_rules WHERE id=?`, rule.ID))
	if errors.Is(err, ErrNotFound) {
		return model.CredentialPromptRule{}, ErrNotFound
	}
	if err != nil {
		return model.CredentialPromptRule{}, err
	}
	rule.CreatedAt = existing.CreatedAt
	rule.SortOrder = existing.SortOrder
	result, err := tx.ExecContext(ctx, `UPDATE credential_prompt_rules SET kind=?,pattern=?,enabled=?,updated_at=? WHERE id=?`,
		rule.Kind, rule.Pattern, boolInt(rule.Enabled), formatTime(rule.UpdatedAt), rule.ID)
	if err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("update credential prompt rule: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("count credential prompt rule: %w", err)
	}
	if count == 0 {
		return model.CredentialPromptRule{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("commit credential prompt rule update: %w", err)
	}
	return rule, nil
}

func (s *Store) DeleteCredentialPromptRule(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM credential_prompt_rules WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete credential prompt rule: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count credential prompt rule: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCredentialPromptRule(row interface{ Scan(...any) error }) (model.CredentialPromptRule, error) {
	var rule model.CredentialPromptRule
	var enabled int
	var created, updated string
	if err := row.Scan(&rule.ID, &rule.Kind, &rule.Pattern, &enabled, &rule.SortOrder, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.CredentialPromptRule{}, ErrNotFound
		}
		return model.CredentialPromptRule{}, fmt.Errorf("scan credential prompt rule: %w", err)
	}
	rule.Enabled = enabled != 0
	parsed, err := parseTime(created)
	if err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("parse credential prompt rule created_at: %w", err)
	}
	rule.CreatedAt = parsed
	parsed, err = parseTime(updated)
	if err != nil {
		return model.CredentialPromptRule{}, fmt.Errorf("parse credential prompt rule updated_at: %w", err)
	}
	rule.UpdatedAt = parsed
	return rule, nil
}
