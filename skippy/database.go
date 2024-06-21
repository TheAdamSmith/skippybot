package skippy

import (
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Database interface {
	CreateGameSession(gs *GameSession) error
	GetGameSession(id uint) (*GameSession, error)
	GetGameSessionsByUser(userID string) (*GameSessions, error)
}

type GameSession struct {
	ID        uint `gorm:"primaryKey"`
	UserID    string
	Game      string
	StartedAt time.Time
	Duration  time.Duration
}
type GameSessions []GameSession

type GameSessionAI struct {
	Game        string
	StartedAt   time.Time
	HoursPlayed string
}

func (gs *GameSessions) ToGameSessionAI() []GameSessionAI {
	var gsai []GameSessionAI
	for _, g := range *gs {
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

func (db *DB) CreateGameSession(gs *GameSession) error {
	return db.DB.Create(gs).Error
}

func (db *DB) GetGameSession(id uint) (*GameSession, error) {
	var gs GameSession
	err := db.DB.First(&gs, id).Error
	return &gs, err
}

func (db *DB) GetGameSessionsByUser(userID string) (*GameSessions, error) {
	var gs *GameSessions
	err := db.DB.Where(&GameSession{UserID: userID}).Find(&gs).Error
	return gs, err
}
