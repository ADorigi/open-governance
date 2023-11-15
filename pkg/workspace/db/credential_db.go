package db

import "github.com/kaytu-io/kaytu-util/pkg/source"

func (s *Database) ListCredentialsByWorkspaceID(id string) ([]Credential, error) {
	var creds []Credential
	err := s.Orm.Model(&Credential{}).
		Where("workspace_id = ?", id).
		Find(&creds).Error
	if err != nil {
		return nil, err
	}
	return creds, nil
}

func (s *Database) CreateCredential(cred *Credential) error {
	err := s.Orm.Model(&Credential{}).
		Create(cred).Error
	if err != nil {
		return err
	}
	return nil
}

func (s *Database) CountConnectionsByConnector(connector source.Type) (int64, error) {
	var count int64
	tx := s.Orm.Raw("select coalesce(sum(connection_count),0) from credentials where connector_type = ?", connector).Find(&count)
	err := tx.Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Database) SetIsCreated(id uint) error {
	tx := s.Orm.
		Model(&Credential{}).
		Where("id = ?", id).
		Update("is_created", true)
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}

func (s *Database) DeleteCredential(id uint) error {
	tx := s.Orm.
		Where("id = ?", id).
		Unscoped().
		Delete(&Credential{})
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}
