// SPDX-License-Identifier: AGPL-3.0-or-later

package folkweave

import (
	"context"
	"strings"

	"github.com/airencracken/comfylib/setup"
)

// HasOwner includes suspended owners so setup never reopens an existing instance.
func (s *Store) HasOwner(ctx context.Context) (bool, error) {
	var present bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE role='owner')").Scan(&present)
	return present, err
}

// CreateFirstOwner creates the browser setup account with an atomic owner guard.
func (s *Store) CreateFirstOwner(ctx context.Context, owner setup.Owner) error {
	owner.Username = strings.ToLower(strings.TrimSpace(owner.Username))
	if err := ValidateCredentials(owner.Username, owner.Password); err != nil {
		return err
	}
	hash, err := HashPassword(owner.Password)
	if err != nil {
		return err
	}
	return s.Bootstrap(ctx, owner.Username, hash, owner.Username, "Our people")
}
