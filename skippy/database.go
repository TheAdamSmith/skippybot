package skippy

import (
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Database interface {
	CreateGameSession(gs *GameSession) error
	GetGameSession(id uint) (*GameSession, error)
	DeleteGameSession(id uint) error
	GetGameSessionsByUser(userID string) ([]GameSession, error)
	GetGameSessionsByUserAndDays(
		userID string,
		daysAgo int,
	) ([]GameSession, error)
	GetGameSessionSum(userID string, daysAgo int) (time.Duration, error)
	Close() error
}

type GameSession struct {
	ID     uint `gorm:"primaryKey"`
	UserID string
	Game   string
	// TODO: this should be stored in utc
	StartedAt time.Time
	Duration  time.Duration
}

type GameSessionAI struct {
	Game        string
	StartedAt   time.Time
	HoursPlayed string
}

func ToGameSessionAI(gs []GameSession) []GameSessionAI {
	var gsai []GameSessionAI
	for _, g := range gs {
		gsai = append(gsai, GameSessionAI{
			Game:        g.Game,
			StartedAt:   g.StartedAt,
			HoursPlayed: g.Duration.String(),
		})
	}
	return gsai
}

type DB struct {
	*gorm.DB
}

func NewDB(dialect, dsn string) (*DB, error) {
	db, err := gorm.Open(sqlite.Open("skippy.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&GameSession{})

	return &DB{db}, nil
}

func (db *DB) Close() error {
	sql, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sql.Close()
}

func (db *DB) CreateGameSession(gs *GameSession) error {
	return db.DB.Create(gs).Error
}

func (db *DB) GetGameSession(id uint) (*GameSession, error) {
	var gs GameSession
	err := db.DB.First(&gs, id).Error
	return &gs, err
}

func (db *DB) DeleteGameSession(id uint) error {
	err := db.DB.Delete(&GameSession{}, id).Error
	return err
}

func (db *DB) GetGameSessionsByUser(userID string) ([]GameSession, error) {
	var gs []GameSession
	err := db.DB.Where(&GameSession{UserID: userID}).Find(&gs).Error
	return gs, err
}

func (db *DB) GetGameSessionsByUserAndDays(
	userID string,
	daysAgo int,
) ([]GameSession, error) {
	var gs []GameSession
	now := time.Now()
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -daysAgo)

	err := db.DB.Where("user_id = ? AND started_at >= ?", userID, cutoff).
		Find(&gs).
		Error
	return gs, err
}

func (db *DB) GetGameSessionSum(userID string, daysAgo int) (time.Duration, error) {
	now := time.Now()
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -daysAgo)

	var totDuration time.Duration
	err := db.Model(&GameSession{}).
		Select("IFNULL(SUM(duration), 0)").
		Where("user_id = ? AND started_at >=?", userID, cutoff).
		Scan(&totDuration).Error

	return totDuration, err
}
