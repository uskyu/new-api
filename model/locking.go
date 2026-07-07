package model

import "gorm.io/gorm"

func lockForUpdate(tx *gorm.DB) *gorm.DB {
	if tx == nil {
		return tx
	}
	if commonUsingSQLite() {
		return tx
	}
	return tx.Set("gorm:query_option", "FOR UPDATE")
}

func commonUsingSQLite() bool {
	return DB != nil && DB.Dialector != nil && DB.Dialector.Name() == "sqlite"
}
