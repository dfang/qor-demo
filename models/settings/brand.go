package settings

import (
	"github.com/jinzhu/gorm"
)

type Brand struct {
	gorm.Model
	Name string
}
