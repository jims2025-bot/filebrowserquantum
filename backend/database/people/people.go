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
	if envPath := os.Getenv("FILEBROWSER_PEOPLE_DB_PATH"); envPath != "" {
		dbPath = envPath
	}

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
		name TEXT UNIQUE NOT NULL COLLATE NOCASE
	);

	CREATE TABLE IF NOT EXISTS face_index (
		person_id INTEGER,
		image_path TEXT NOT NULL,
		confidence REAL,
		box TEXT,
		is_avatar INTEGER DEFAULT 0,
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
	CREATE INDEX IF NOT EXISTS idx_face_index_path ON face_index(image_path);
	CREATE INDEX IF NOT EXISTS idx_face_embeddings_person ON face_embeddings(person_id);
	`
	_, err = db.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: Add box column if it doesn't exist
	_, _ = db.Exec("ALTER TABLE face_index ADD COLUMN box TEXT")
	// Migration: Add is_avatar column (default 0)
	_, _ = db.Exec("ALTER TABLE face_index ADD COLUMN is_avatar INTEGER DEFAULT 0")

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
func MapFaceToIndex(name string, imagePath string, confidence float64, box string) error {
	if DB == nil {
		return nil // Graceful skip if DB failed to init
	}

	personID, err := GetOrCreatePerson(name)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		INSERT OR REPLACE INTO face_index (person_id, image_path, confidence, box) 
		VALUES (?, ?, ?, ?)
	`, personID, imagePath, confidence, box)

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

func SetPersonAvatar(name string, imagePath string, box string) error {
	if DB == nil {
		return nil
	}
	personID, err := GetOrCreatePerson(name)
	if err != nil {
		return err
	}

	// 1. Reset all avatars for this person
	_, err = DB.Exec("UPDATE face_index SET is_avatar = 0 WHERE person_id = ?", personID)
	if err != nil {
		return err
	}

	// 2. Set the new avatar
	_, err = DB.Exec(`
		UPDATE face_index 
		SET is_avatar = 1 
		WHERE person_id = ? AND image_path = ? AND box = ?
	`, personID, imagePath, box)
	return err
}

type PersonMatch struct {
	ImagePath string `json:"imagePath"`
	Box       string `json:"box"`
}

// SearchImagesByPerson returns all absolute image paths tagged with a specific person name.
func SearchImagesByPerson(nameQuery string, pathPrefix string, limit int, offset int) ([]PersonMatch, error) {
	if DB == nil {
		return nil, fmt.Errorf("people.db not initialized")
	}

	queryStr := `
		SELECT fi.image_path, fi.box 
		FROM face_index fi
		JOIN people p ON p.id = fi.person_id
		WHERE p.name = ?
	`
	args := []interface{}{nameQuery}

	if pathPrefix != "" {
		// Ensure pathPrefix ends with separator for subfolder matching
		prefix := filepath.Clean(pathPrefix)
		queryStr += " AND (fi.image_path = ? OR fi.image_path LIKE ?)"
		args = append(args, prefix, prefix+string(filepath.Separator)+"%")
	}

	queryStr += `
		ORDER BY fi.is_avatar DESC, fi.confidence DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, limit, offset)

	rows, err := DB.Query(queryStr, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PersonMatch
	for rows.Next() {
		var m PersonMatch
		var box sql.NullString
		if err := rows.Scan(&m.ImagePath, &box); err == nil {
			if box.Valid {
				m.Box = box.String
			}
			results = append(results, m)
		}
	}

	return results, nil
}

// SearchImagesByPeople returns images matching a set of people using AND/OR logic.
func SearchImagesByPeople(names []string, logic string, pathPrefix string, limit int, offset int) ([]PersonMatch, error) {
	if DB == nil {
		return nil, fmt.Errorf("people.db not initialized")
	}
	if len(names) == 0 {
		return []PersonMatch{}, nil
	}

	var queryStr string
	var args []interface{}

	placeholders := make([]string, len(names))
	for i := range names {
		placeholders[i] = "?"
		args = append(args, names[i])
	}
	placeholderStr := strings.Join(placeholders, ",")

	if strings.ToUpper(logic) == "AND" {
		queryStr = fmt.Sprintf(`
			SELECT fi.image_path, fi.box
			FROM face_index fi
			JOIN people p ON p.id = fi.person_id
			WHERE fi.image_path IN (
				SELECT fi2.image_path
				FROM face_index fi2
				JOIN people p2 ON p2.id = fi2.person_id
				WHERE p2.name IN (%s)
				GROUP BY fi2.image_path
				HAVING COUNT(DISTINCT p2.id) = ?
			)
			AND p.name = ?
		`, placeholderStr)
		args = append(args, len(names), names[0]) // Get box for the first person
	} else {
		// OR logic
		queryStr = fmt.Sprintf(`
			SELECT fi.image_path, fi.box 
			FROM face_index fi
			JOIN people p ON p.id = fi.person_id
			WHERE p.name IN (%s)
		`, placeholderStr)
	}

	if pathPrefix != "" {
		prefix := filepath.Clean(pathPrefix)
		queryStr += " AND (fi.image_path = ? OR fi.image_path LIKE ?)"
		args = append(args, prefix, prefix+string(filepath.Separator)+"%")
	}

	if strings.ToUpper(logic) != "AND" {
		queryStr += " GROUP BY fi.image_path "
	}

	queryStr += `
		ORDER BY fi.is_avatar DESC, fi.confidence DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, limit, offset)

	rows, err := DB.Query(queryStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PersonMatch
	for rows.Next() {
		var m PersonMatch
		var box sql.NullString
		if err := rows.Scan(&m.ImagePath, &box); err == nil {
			if box.Valid {
				m.Box = box.String
			}
			results = append(results, m)
		}
	}
	return results, nil
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

// PersonSummary represents a person with their associated face count and an avatar URL.
type PersonSummary struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	AvatarUrl string `json:"avatarUrl"`
}

// GetPeopleSummary returns a list of unique people and the number of indexed faces for each.
func GetPeopleSummary(pathPrefix string) ([]PersonSummary, error) {
	if DB == nil {
		return nil, fmt.Errorf("people.db not initialized")
	}

	prefixMatch := ""
	if pathPrefix != "" {
		prefix := filepath.Clean(pathPrefix)
		prefixMatch = fmt.Sprintf("WHERE image_path = '%s' OR image_path LIKE '%s%c%%'", prefix, prefix, filepath.Separator)
	}

	rows, err := DB.Query(fmt.Sprintf(`
		SELECT name, cnt, box, image_path
		FROM (
			SELECT p.name, counts.cnt, fi.box, fi.image_path,
			       ROW_NUMBER() OVER (PARTITION BY p.id ORDER BY fi.is_avatar DESC, fi.confidence DESC) as rn
			FROM people p
			JOIN (
				SELECT person_id, COUNT(*) as cnt
				FROM face_index
				%s
				GROUP BY person_id
			) counts ON p.id = counts.person_id
			JOIN face_index fi ON fi.person_id = p.id
			%s
		) WHERE rn = 1
		ORDER BY cnt DESC, name ASC
	`, prefixMatch, prefixMatch))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PersonSummary
	for rows.Next() {
		var s PersonSummary
		var box, path sql.NullString
		if err := rows.Scan(&s.Name, &s.Count, &box, &path); err == nil {
			if path.Valid {
				boxStr := ""
				if box.Valid {
					boxStr = box.String
				}
				// Always provide an AvatarUrl if we have a path
				s.AvatarUrl = fmt.Sprintf("RAW:%s?box=%s", path.String, boxStr)
			}
			results = append(results, s)
		}
	}
	return results, nil
}
