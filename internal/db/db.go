package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Insight struct {
	ID          int
	PlayerName  string
	MatchName   string
	Round       int
	Tick        int
	Type        string // e.g., "PrematureFire", "Spasm", "SprayEfficiency", "Gunfight"
	Severity    string // "Low", "Medium", "High"
	Description string
	Metadata    string // JSON encoded metadata for rich UI rendering
	CreatedAt   time.Time
}

type Database struct {
	db *sql.DB
}

func InitDB(filepath string) (*Database, error) {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}

	err = createTables(db)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func createTables(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS insights (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		player_name TEXT,
		match_name TEXT,
		round INTEGER,
		tick INTEGER,
		type TEXT,
		severity TEXT,
		description TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	return err
}

func (d *Database) ClearInsights() error {
	_, err := d.db.Exec("DELETE FROM insights")
	return err
}

func (d *Database) SaveInsight(i Insight) error {
	query := `
	INSERT INTO insights (player_name, match_name, round, tick, type, severity, description, metadata)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.Exec(query, i.PlayerName, i.MatchName, i.Round, i.Tick, i.Type, i.Severity, i.Description, i.Metadata)
	return err
}

func (d *Database) GetInsightsForPlayer(playerName string) ([]Insight, error) {
	query := `SELECT id, player_name, match_name, round, tick, type, severity, description, metadata, created_at 
			  FROM insights WHERE player_name = ? ORDER BY id DESC`
	rows, err := d.db.Query(query, playerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var insights []Insight
	for rows.Next() {
		var i Insight
		err := rows.Scan(&i.ID, &i.PlayerName, &i.MatchName, &i.Round, &i.Tick, &i.Type, &i.Severity, &i.Description, &i.Metadata, &i.CreatedAt)
		if err != nil {
			return nil, err
		}
		insights = append(insights, i)
	}
	return insights, nil
}
