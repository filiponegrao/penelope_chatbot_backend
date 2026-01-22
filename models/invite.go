package models

import "time"

/************************************************
/**** MARK: INVITE TYPES  ****/
/************************************************/
const INVITE_STATUS_PENDING = 0
const INVITE_STATUS_VALIDATED = 1
const INVITE_STATUS_EXPIRED = 2

type Invite struct {
	ID        int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	Inviter   User       `json:"inviter" form:"inviter" gorm:"association_autoupdate:false;association_autocreate:false; not null"`
	InviterID int64      `json:"inviter_id" form:"inviter_id" gorm:"not null"`
	Invited   User       `json:"invited" form:"invited" gorm:"association_autoupdate:false;association_autocreate:false; not null"`
	InvitedID int64      `json:"invited_id" form:"invited_id" gorm:"not null"`
	Code      string     `json:"code" form:"code" gorm:"not null;unique"`
	Status    int64      `json:"status" form:"status" gorm:"default:0"`
	ExpiresAt *time.Time `json:"expires_at" form:"expires_at"`
	CreatedAt *time.Time `json:"created_at" form:"created_at"`
	UpdatedAt *time.Time `json:"updated_at" form:"updated_at"`
}

func (invite Invite) MissingFields() string {
	if invite.InviterID == 0 {
		return "inviter_id"
	} else if invite.InvitedID == 0 {
		return "invited_id"
	}
	return ""
}
