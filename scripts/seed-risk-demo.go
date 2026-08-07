package main

import (
	"fmt"
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type seedUser struct {
	Id          int    `gorm:"primaryKey"`
	Username    string `gorm:"uniqueIndex;not null"`
	Password    string `gorm:"not null"`
	DisplayName string
	Role        int    `gorm:"default:1"`
	Status      int    `gorm:"default:1"`
	Group       string `gorm:"type:varchar(64);default:'default'"`
	AffCode     string `gorm:"type:varchar(32);uniqueIndex"`
	InviterId   int
	CreatedAt   int64 `gorm:"autoCreateTime"`
}

func (seedUser) TableName() string {
	return "users"
}

type seedRiskIPRecord struct {
	Id          int    `gorm:"primaryKey"`
	UserId      int    `gorm:"uniqueIndex:uidx_risk_ip_user_token_source;index"`
	TokenId     int    `gorm:"uniqueIndex:uidx_risk_ip_user_token_source;index"`
	IP          string `gorm:"type:varchar(64);uniqueIndex:uidx_risk_ip_user_token_source;index"`
	Source      string `gorm:"type:varchar(16);uniqueIndex:uidx_risk_ip_user_token_source;index"`
	FirstSeenAt int64  `gorm:"bigint;index"`
	LastSeenAt  int64  `gorm:"bigint;index"`
	EventCount  int64  `gorm:"default:0"`
}

func (seedRiskIPRecord) TableName() string {
	return "risk_ip_records"
}

func main() {
	db, err := gorm.Open(sqlite.Open("one-api.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if err := db.Exec("DELETE FROM risk_ip_records WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'risk-demo-%')").Error; err != nil {
		log.Fatalf("failed to clean old risk records: %v", err)
	}
	if err := db.Unscoped().Where("username LIKE ?", "risk-demo-%").Delete(&seedUser{}).Error; err != nil {
		log.Fatalf("failed to clean old demo users: %v", err)
	}

	const userCount = 45
	users := make([]seedUser, 0, userCount)
	for i := 1; i <= userCount; i++ {
		inviterId := 0
		if i > 6 {
			inviterId = (i-7)%6 + 1
		}
		users = append(users, seedUser{
			Username:    fmt.Sprintf("risk-demo-%03d", i),
			Password:    "risk-demo-pass",
			DisplayName: fmt.Sprintf("风控测试用户%02d", i),
			Role:        1,
			Status:      1,
			Group:       "default",
			AffCode:     fmt.Sprintf("risk-demo-aff-%03d", i),
			InviterId:   inviterId,
			CreatedAt:   time.Now().Unix(),
		})
	}
	if err := db.Create(&users).Error; err != nil {
		log.Fatalf("failed to create demo users: %v", err)
	}

	now := time.Now().Unix()
	records := make([]seedRiskIPRecord, 0, userCount*2)
	for index, user := range users {
		groupIndex := index / 3
		sharedIP := fmt.Sprintf("198.51.100.%d", groupIndex+10)
		records = append(records, seedRiskIPRecord{
			UserId: user.Id, TokenId: index + 1, IP: sharedIP,
			Source: "token", FirstSeenAt: now - int64(userCount-index), LastSeenAt: now, EventCount: int64(index%5 + 2),
		})
		records = append(records, seedRiskIPRecord{
			UserId: user.Id, TokenId: 0, IP: fmt.Sprintf("203.0.113.%d", index+1),
			Source: "login", FirstSeenAt: now - int64(userCount-index), LastSeenAt: now, EventCount: int64(index%7 + 1),
		})
	}
	if err := db.Create(&records).Error; err != nil {
		log.Fatalf("failed to create risk records: %v", err)
	}

	fmt.Printf("seeded %d demo users, %d risk records\n", len(users), len(records))
}