package db

import (
	"context"
	"fmt"

	pgx "github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
	"github.com/karutselvan/chat-assistant/server/env"
)

type EmbeddingsDB interface {
	// Adds enbeddings and text into the LLM memory
	Add(author int64, text string, embeddings []float32) (int64, error)
	// Finds the N closest matches
	Find(embedding []float32, count int) ([]string, error)

	Close()
}

type NoopEmbeddingsDB struct{}

func (n NoopEmbeddingsDB) Close() {}
func (n NoopEmbeddingsDB) Add(author int64, text string, embeddings []float32) (int64, error) {
	return 0, nil
}
func (n NoopEmbeddingsDB) Find(embedding []float32, count int) ([]string, error) {
	return []string{}, nil
}

type AuthorsDB interface {
	// Adds an author into the author database
	Add(author int64, email string, name string) (int64, error)

	Close()
}

type PostgresDatabase struct {
	ctx  context.Context
	conn *pgx.Conn
}

// returns chunk id
func (emb *PostgresDatabase) Add(author int64, text string, embeddings []float32) (int64, error) {
	sql := `
INSERT INTO embeddings(
	content, tokens, author, created, embedding
) VALUES(
	$1, $2, $3, NOW(), $4
) RETURNING id;`
	var id int64
	if err := emb.conn.QueryRow(emb.ctx, sql, text, 0, author, pgvector.NewVector(embeddings)).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// Finds the count closes matches and returns the text
// / TODO - we'll likely want author, text, and other metadata later, use struct
func (emb *PostgresDatabase) Find(embedding []float32, count int) ([]string, error) {
	// Query: SELECT content, 1 - (embedding <=> $1) AS cosine_similarity FROM embeddings ORDER BY 2 DESC
	sql := `SELECT content
	FROM  embeddings 
	ORDER BY embedding <=> $1
	LIMIT $2;
`
	rows, err := emb.conn.Query(emb.ctx, sql, pgvector.NewVector(embedding), count)
	var text string
	results := make([]string, 0, count)
	if err != nil {
		return nil, err
	}
	pgx.ForEachRow(rows, []any{&text}, func() error {
		results = append(results, text)
		return nil
	})
	return results, nil
}

func (emb *PostgresDatabase) Close() {
	emb.conn.Close(emb.ctx)
}

func NewPostgresDatabase(env *env.Environment) (*PostgresDatabase, error) {
	// urlExample := "postgres://username:password@localhost:5432/database_name"
	/*dbname := env.DatabaseDatabase
	if len(dbname) == 0 {
		dbname = "assistant"
	}*/
	dbport := 5432
	dbUserName := "postgres"
        dbPassword := "psql1234"
        dbHostname := "34.123.205.223"
        dbname := "chat-assistant"

	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", dbUserName, dbPassword, dbHostname, dbport, dbname)
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return &PostgresDatabase{
		ctx:  context.Background(),
		conn: conn,
	}, err

}
