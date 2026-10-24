package session

import "context"

func (s *service) ListChildren(ctx context.Context, parentID string) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM sessions WHERE parent_session_id = ? ORDER BY created_at, id", parentID)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	children := make([]Session, 0, len(ids))
	for _, id := range ids {
		child, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return children, nil
}
