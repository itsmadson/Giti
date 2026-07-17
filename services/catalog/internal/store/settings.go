package store

import "context"

// GetSettings returns the raw global-settings JSON blob.
func (s *Store) GetSettings(ctx context.Context) ([]byte, error) {
	var cfg []byte
	err := s.db.QueryRow(ctx, `SELECT config FROM settings WHERE id=1`).Scan(&cfg)
	if err != nil {
		return []byte("{}"), mapErr(err)
	}
	return cfg, nil
}

// SetSettings replaces the global-settings JSON blob.
func (s *Store) SetSettings(ctx context.Context, cfg []byte) error {
	_, err := s.db.Exec(ctx, `UPDATE settings SET config=$1 WHERE id=1`, cfg)
	return err
}
