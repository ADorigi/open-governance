package repository

import (
	"context"
	"errors"

	"github.com/kaytu-io/kaytu-engine/services/integration/db"
	"github.com/kaytu-io/kaytu-engine/services/integration/model"
	"gorm.io/gorm/clause"
)

var (
	ErrDuplicateCredential = errors.New("didn't create credential due to id conflict")
	ErrCredentialNotFound  = errors.New("cannot find the given credential")
)

type Credential interface {
	Create(context.Context, *model.Credential) error
	Update(context.Context, *model.Credential) error
}

type CredentialSQL struct {
	db db.Database
}

func NewCredentialSQL(db db.Database) Credential {
	return CredentialSQL{
		db: db,
	}
}

func (c CredentialSQL) Create(ctx context.Context, cred *model.Credential) error {
	tx := c.db.DB.
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(cred)

	if tx.Error != nil {
		return tx.Error
	} else if tx.RowsAffected == 0 {
		return ErrDuplicateCredential
	}

	return nil
}

func (c CredentialSQL) Update(ctx context.Context, creds *model.Credential) error {
	tx := c.db.DB.WithContext(ctx).
		Model(&model.Credential{}).
		Where("id = ?", creds.ID.String()).Updates(creds)

	if tx.Error != nil {
		return tx.Error
	} else if tx.RowsAffected != 1 {
		return ErrCredentialNotFound
	}

	return nil
}
