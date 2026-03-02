package people

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// InitDB initializes the SQLite database for facial recognition.
func InitDB(basePath string) error {
	dbPath := filepath.Join(basePath, "people.db")

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// Create tables if they don't exist
	schema := `
	CREATE TABLE IF NOT EXISTS people (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS face_index (
		person_id INTEGER,
		image_path TEXT NOT NULL,
		confidence REAL,
		FOREIGN KEY(person_id) REFERENCES people(id),
		UNIQUE(person_id, image_path)
	);

	CREATE TABLE IF NOT EXISTS face_embeddings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		person_id INTEGER,
		image_path TEXT NOT NULL,
		embedding BLOB NOT NULL,
		FOREIGN KEY(person_id) REFERENCES people(id)
	);

	CREATE INDEX IF NOT EXISTS idx_face_index_person ON face_index(person_id);
	CREATE INDEX IF NOT EXISTS idx_face_embeddings_person ON face_embeddings(person_id);
	`
	_, err = db.Exec(schema)
	if err != nil {
		return err
	}

	DB = db
	log.Println("Initialized people.db for Facial Recognition Indexing")
	return nil
}

// GetOrCreatePerson returns the ID of a person, creating them if they don't exist.
func GetOrCreatePerson(name string) (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("people.db not initialized")
	}

	// Try to insert (Ignores if UNIQUE constraint fails)
	_, err := DB.Exec("INSERT OR IGNORE INTO people (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}

	// Fetch the ID
	var id int64
	err = DB.QueryRow("SELECT id FROM people WHERE name = ?", name).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// MapFaceToIndex links a person to an image path.
func MapFaceToIndex(name string, imagePath string, confidence float64) error {
	if DB == nil {
		return nil // Graceful skip if DB failed to init
	}

	personID, err := GetOrCreatePerson(name)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		INSERT OR REPLACE INTO face_index (person_id, image_path, confidence) 
		VALUES (?, ?, ?)
	`, personID, imagePath, confidence)

	return err
}

// RemoveFaceFromIndex deletes a link between a person and an image (used during manual UI removal).
func RemoveFaceFromIndex(name string, imagePath string) error {
	if DB == nil {
		return nil
	}

	_, err := DB.Exec(`
		DELETE FROM face_index 
		WHERE image_path = ? AND person_id = (SELECT id FROM people WHERE name = ?)
	`, imagePath, name)

	return err
}

// SearchImagesByPerson returns all absolute image paths tagged with a specific person name.
func SearchImagesByPerson(nameQuery string) ([]string, error) {
	if DB == nil {
		return nil, fmt.Errorf("people.db not initialized")
	}

	// Basic wildcard search
	searchTerm := "%" + strings.ToLower(nameQuery) + "%"

	rows, err := DB.Query(`
		SELECT fi.image_path 
		FROM face_index fi
		JOIN people p ON p.id = fi.person_id
		WHERE LOWER(p.name) LIKE ?
		ORDER BY fi.confidence DESC
	`, searchTerm)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			paths = append(paths, p)
		}
	}

	return paths, nil
}

// SaveFaceEmbedding stores a serialized embedding for a person/image combo.
func SaveFaceEmbedding(name string, imagePath string, embedding []float64) error {
	if DB == nil {
		return nil
	}

	personID, err := GetOrCreatePerson(name)
	if err != nil {
		return err
	}

	// Serialize embedding to binary (JSON is easiest for simplicity)
	b, err := json.Marshal(embedding)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		INSERT OR REPLACE INTO face_embeddings (person_id, image_path, embedding) 
		VALUES (?, ?, ?)
	`, personID, imagePath, b)

	return err
}

type PersonEmbedding struct {
	Name      string
	Embedding []float64
}

// GetAllEmbeddings returns everything in the DB for similarity search.
func GetAllEmbeddings() ([]PersonEmbedding, error) {
	if DB == nil {
		return nil, nil
	}

	rows, err := DB.Query(`
		SELECT p.name, fe.embedding 
		FROM face_embeddings fe
		JOIN people p ON p.id = fe.person_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PersonEmbedding
	for rows.Next() {
		var name string
		var b []byte
		if err := rows.Scan(&name, &b); err == nil {
			var emb []float64
			if err := json.Unmarshal(b, &emb); err == nil {
				results = append(results, PersonEmbedding{Name: name, Embedding: emb})
			}
		}
	}
	return results, nil
}
