package models

import (
	"penelope/tools"
	"strings"
	"time"
)

const USER_GENDER_MALE = "male"
const USER_GENDER_FEMALE = "female"
const USER_GENDER_OTHER = "other"

/************************************************
/**** MARK: USER TYPES ****/
/************************************************/
const USER_TYPE_NORMAL = 0
const USER_TYPE_ADMIN = 1
const USER_TYPE_MANAGER = 2
const USER_TYPE_CONTROLLER = 3
const USER_TYPE_STOCKMAN = 4
const USER_TYPE_DELIVERY = 5

/************************************************
/**** MARK: USER STATUS ****/
/************************************************/
const USER_STATUS_AVAILABLE = 0
const USER_STATUS_PENDING = 1
const USER_STATUS_BLOCKED = 2

// User representa um usuario no sistema
type User struct {
	ID                  int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	Name                string     `gorm:"not null" json:"name" form:"name"`
	Email               string     `gorm:"not null;unique" json:"email" form:"email"`
	Password            string     `gorm:"not null" json:"password" form:"password"`
	Gender              string     `gorm:"not null" json:"gender" form:"gender"`
	FacebookID          string     `gorm:"column:facebook_id" json:"facebook_id" form:"facebook_id"`
	AddressPostal       string     `gorm:"column:address_postal" json:"address_postal" form:"address_postal"`
	AddressState        string     `gorm:"column:address_state" json:"address_state" form:"address_state"`
	AddressCity         string     `gorm:"column:address_city" json:"address_city" form:"address_city"`
	AddressNeighborhood string     `gorm:"column:address_neighborhood" json:"address_neighborhood" form:"address_neighborhood"`
	AddressStreet       string     `gorm:"column:address_street" json:"address_street" form:"address_street"`
	AddressNumber       int        `gorm:"column:address_number" json:"address_number" form:"address_number"`
	AddressComplement   string     `gorm:"column:address_complement" json:"address_complement" form:"address_complement"`
	CNPJ                string     `json:"cnpj" form:"cnpj"`
	CPF                 string     `gorm:"default:''" json:"cpf" form:"cpf"`
	Birthdate           string     `gorm:"default:''" json:"birthdate" form:"birthdate"`
	Phone1              string     `gorm:"column:phone1" json:"phone1" form:"phone1"`
	Phone2              string     `gorm:"column:phone2" json:"phone2" form:"phone2"`
	ProfileImageURL     string     `gorm:"column:profile_image_url" json:"profile_image_url" form:"profile_image_url"`
	Status              int        `gorm:"default:0" json:"status" form:"status"`
	Type                int        `gorm:"not null; default:0" json:"type" form:"type"`
	Info                string     `gorm:"not null" json:"info" form:"info"`
	StateRegistration   string     `json:"state_registration" form:"state_registration"`
	IsMEI               bool       `json:"is_mei" form:"is_mei"`
	Role                string     `json:"role" gorm:"role"`
	Site                string     `gorm:"default:''" json:"site" form:"site"`
	Token               string     `gorm:"default:''" json:"token" form:"token"`
	Admin               bool       `gorm:"not null; default: false" json:"admin" form:"admin"`
	Platform            string     `gorm:"default:''" json:"platform" form:"platform"`
	CreatedAt           *time.Time `json:"created_at" form:"created_at"`
	UpdatedAt           *time.Time `json:"updated_at" form:"updated_at"`
}

func (user User) MissingFields() string {
	if user.Name == "" {
		return "name"
	} else if user.Email == "" {
		return "email"
	} else if user.Password == "" {
		return "password"
	} else if tools.CheckPassword(user.Password) != "" {
		return tools.CheckPassword(user.Password)
	} else if user.Phone1 == "" {
		return "phone1"
	}
	return ""
}

func IsCpfValid(cpf string) bool {
	if cpf == "" {
		return false
	} else if strings.Count(cpf, "") != 12 {
		return false
	}
	return true
}

func IsCnpjValid(cnpj string) bool {
	if cnpj == "" {
		return false
	} else if strings.Count(cnpj, "") != 15 {
		return false
	}
	return true
}
