package main

import (
	"fmt"
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
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
	Quota       int64 `gorm:"default:0"`
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

type seedCheckin struct {
	Id           int    `gorm:"primaryKey"`
	UserId       int    `gorm:"uniqueIndex:idx_user_checkin_date"`
	CheckinDate  string `gorm:"type:varchar(10);uniqueIndex:idx_user_checkin_date"`
	QuotaAwarded int
	CreatedAt    int64
}

func (seedCheckin) TableName() string {
	return "checkins"
}

type seedLog struct {
	Id          int    `gorm:"primaryKey"`
	UserId      int    `gorm:"index"`
	CreatedAt   int64  `gorm:"bigint"`
	Type        int    `gorm:"index"`
	Content     string
	Username    string `gorm:"default:''"`
	TokenName   string `gorm:"default:''"`
	ModelName   string `gorm:"default:''"`
	Quota       int    `gorm:"default:0"`
	IsStream    bool
	ChannelId   int `gorm:"default:0"`
	TokenId     int `gorm:"default:0"`
	Group       string `gorm:"default:''"`
	Ip          string `gorm:"default:''"`
	RequestId   string `gorm:"default:''"`
	UseTime     int    `gorm:"default:0"`
	PromptToks  int    `gorm:"column:prompt_tokens;default:0"`
	CompletionToks int `gorm:"column:completion_tokens;default:0"`
	UpstreamRequestId string `gorm:"default:''"`
	Other       string `gorm:"default:''"`
}

func (seedLog) TableName() string {
	return "logs"
}

type seedOption struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

func (seedOption) TableName() string {
	return "options"
}

const checkinTiers = `[{"threshold":20,"min_quota":5000,"max_quota":20000},{"threshold":100,"min_quota":20000,"max_quota":100000}]`

var checkinOptions = map[string]string{
	"checkin_setting.enabled":        "true",
	"checkin_setting.min_quota":      "1000",
	"checkin_setting.max_quota":      "10000",
	"checkin_setting.captcha_enabled": "true",
	"checkin_setting.captcha_kind":   "math",
	"checkin_setting.bonus_enabled":  "true",
	"checkin_setting.bonus_metric":   "request_count",
	"checkin_setting.bonus_tiers":    checkinTiers,
}

// yesterdayCalls 每个用户昨日模拟的调用次数（用于命中活跃阶梯档位）
var yesterdayCalls = []int{5, 12, 25, 40, 60, 120, 150, 220, 300, 18, 8, 3, 55, 90, 130, 200, 45, 70, 110, 160, 12, 6, 35, 80, 140, 250, 30, 22, 18, 28, 5, 45, 65, 95, 155, 210, 40, 25, 15, 9, 7, 33, 50, 75, 115}

// ensureRoot 重置管理员账号为 root / 12345678，保证用户可以直接登录测试
func ensureRoot(db *gorm.DB) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte("12345678"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	var root seedUser
	if err := db.Where("role = ?", 100).First(&root).Error; err == nil {
		root.Username = "root"
		root.Password = string(hashed)
		root.DisplayName = "Root User"
		root.Status = 1
		root.Quota = 100000000
		return db.Save(&root).Error
	}
	root = seedUser{
		Username: "root", Password: string(hashed), DisplayName: "Root User",
		Role: 100, Status: 1, Group: "default", Quota: 100000000,
		AffCode: "seed-root-aff", CreatedAt: time.Now().Unix(),
	}
	return db.Create(&root).Error
}

func main() {
	db, err := gorm.Open(sqlite.Open("one-api.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if err := ensureRoot(db); err != nil {
		log.Fatalf("failed to reset root account: %v", err)
	}
	fmt.Println("root account reset to root / 12345678")

	// 幂等清理：删除之前的演示数据
	var demoUserIds []int
	if err := db.Table("users").Where("username LIKE ?", "risk-demo-%").Pluck("id", &demoUserIds).Error; err != nil {
		log.Fatalf("failed to query old demo users: %v", err)
	}
	if len(demoUserIds) > 0 {
		if err := db.Exec("DELETE FROM risk_ip_records WHERE user_id IN ?", demoUserIds).Error; err != nil {
			log.Fatalf("failed to clean old risk records: %v", err)
		}
		if err := db.Exec("DELETE FROM checkins WHERE user_id IN ?", demoUserIds).Error; err != nil {
			log.Fatalf("failed to clean old checkin records: %v", err)
		}
		if err := db.Exec("DELETE FROM logs WHERE user_id IN ?", demoUserIds).Error; err != nil {
			log.Fatalf("failed to clean old demo logs: %v", err)
		}
	}
	if err := db.Unscoped().Where("username LIKE ?", "risk-demo-%").Delete(&seedUser{}).Error; err != nil {
		log.Fatalf("failed to clean old demo users: %v", err)
	}

	// 写入签到选项（幂等 upsert）
	for key, value := range checkinOptions {
		option := seedOption{Key: key}
		if err := db.Where(seedOption{Key: key}).FirstOrCreate(&option).Error; err != nil {
			log.Fatalf("failed to create option %s: %v", key, err)
		}
		option.Value = value
		if err := db.Save(&option).Error; err != nil {
			log.Fatalf("failed to update option %s: %v", key, err)
		}
	}

	const userCount = 45
	demoPassHash, err := bcrypt.GenerateFromPassword([]byte("risk-demo-pass"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash demo password: %v", err)
	}
	users := make([]seedUser, 0, userCount)
	for i := 1; i <= userCount; i++ {
		inviterId := 0
		if i > 6 {
			inviterId = (i-7)%6 + 1
		}
		users = append(users, seedUser{
			Username:    fmt.Sprintf("risk-demo-%03d", i),
			Password:    string(demoPassHash),
			DisplayName: fmt.Sprintf("风控测试用户%02d", i),
			Role:        1,
			Status:      1,
			Group:       "default",
			AffCode:     fmt.Sprintf("risk-demo-aff-%03d", i),
			InviterId:   inviterId,
			Quota:       1000000,
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

	// 昨日 00:00 ~ 今天 00:00 时间窗，用于签到活跃档位统计
	today := time.Now()
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location()).Add(-24 * time.Hour).Unix()
	end := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location()).Unix()

	logs := make([]seedLog, 0, userCount)
	for index, user := range users {
		calls := yesterdayCalls[index]
		ts := start + int64(index*3600/(userCount)) // 均匀分布到昨日
		for j := 0; j < calls; j++ {
			logs = append(logs, seedLog{
				UserId:    user.Id,
				CreatedAt: ts + int64(j%300),
				Type:      2, // LogTypeConsume
				Content:   "seed demo usage",
				Username:  user.Username,
				TokenName: "seed-token",
				ModelName: "gpt-4o-mini",
				Quota:     1000 + (j%9)*100,
				UseTime:   500,
				ChannelId: 1,
				TokenId:   1,
				Group:     "default",
				Ip:        "198.51.100.1",
			})
		}
	}
	if err := db.CreateInBatches(&logs, 500).Error; err != nil {
		log.Fatalf("failed to create demo usage logs: %v", err)
	}

	// 签到记录：前 20 个用户今天已签到（可测"今日已签到"态），部分用户近几天连续签到
	checkins := make([]seedCheckin, 0, 40)
	todayStr := time.Now().Format("2006-01-02")
	for index, user := range users {
		if index >= 20 {
			break
		}
		checkins = append(checkins, seedCheckin{
			UserId:       user.Id,
			CheckinDate:  todayStr,
			QuotaAwarded: 5000 + index*100,
			CreatedAt:    end - 3600,
		})
		if index%4 == 0 {
			checkins = append(checkins, seedCheckin{
				UserId:       user.Id,
				CheckinDate:  time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
				QuotaAwarded: 3000 + index*50,
				CreatedAt:    start + 3600,
			})
		}
	}
	if err := db.Create(&checkins).Error; err != nil {
		log.Fatalf("failed to create demo checkins: %v", err)
	}

	fmt.Printf("seeded %d demo users, %d risk records, %d usage logs, %d checkins, %d checkin options\n",
		len(users), len(records), len(logs), len(checkins), len(checkinOptions))
	fmt.Println("checkin options written to `options` table; restart the backend (or start it after seeding) so the in-memory config picks them up.")
}
