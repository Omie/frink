package frink

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	// import postgres driver
	_ "github.com/lib/pq"
)

var specialChars = "`~!@#$%^&*()-_+=|\\{}[]:;'\"/?.,><"
var dbQuery = "SELECT name, similarity(name, '%s') AS sml FROM countries WHERE name ILIKE '%s' ORDER BY sml DESC, name"
var suggestionFormat = "<i>%s</i>"

// Frink type is a core type to bind methods to
type Frink struct{}

type token struct {
	Original        string
	Suggestion      string
	SuggestionScore float32
	Order           int
}

func (t *token) GetSuggestionFromDB(db *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()
	/*
		check Original is worth getting spell-checked, it's worth if
			- length >= 4
			- return if not
		query the db
		if suggestion score >= 0.300, put in the Suggestion from db value
		else, copy the original value to suggestion
	*/
	if len(t.Original) < 4 {
		t.Suggestion = t.Original
		t.SuggestionScore = 0.0
		return
	}

	tokenQuery := fmt.Sprintf(dbQuery, t.Original, "%"+t.Original+"%")
	err := db.QueryRow(tokenQuery).Scan(&t.Suggestion, &t.SuggestionScore)
	switch {
	case err == sql.ErrNoRows:
		t.Suggestion = t.Original
		t.SuggestionScore = 0.0
		// fmt.Println("ERROR: NoRows")
	case err != nil:
		t.Suggestion = t.Original
		t.SuggestionScore = 0.0
		fmt.Println("ERROR: ", err.Error())
	}
	if t.SuggestionScore < 0.3 {
		t.Suggestion = t.Original
	}
}

func cleanQuery(query string) string {
	for _, ch := range specialChars {
		chWithSpace := fmt.Sprintf(" %c ", ch)
		query = strings.Replace(query, string(ch), chWithSpace, -1)
	}
	return query
}

func createTokens(cleandQuery string) []token {
	var tokens []token
	parts := strings.Split(cleandQuery, " ")
	for idx, part := range parts {
		t := token{Original: part, Order: idx + 1}
		tokens = append(tokens, t)
	}
	return tokens
}

func getDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", "user=omie password=omkarnath dbname=countries sslmode=disable")
	if err != nil {
		return nil, err
	}
	return db, nil
}

// GetSuggestion takes a query as an input and returns a corrected query
func (f *Frink) GetSuggestion(query string, format bool) (string, error) {
	/*
		Expect query to be something like a question, for example,
			"who is the president of united states of america?"
		Steps:
			- clean the complete string, replace special characters with "<space>character"
			- split the query on space and create ordered tokens
			- for each token, query database for spell-check in async mode
				- store result in the token and return
			- wait for all tokens to finish
			- create a new query string by replacing the suggestions in the token - if given
			- return the string
	*/
	cleanedQuery := cleanQuery(query)

	tokens := createTokens(cleanedQuery)

	db, err := getDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	// query database to get spell-check for each token
	wg := sync.WaitGroup{}
	wg.Add(len(tokens))
	for idx := range tokens {
		go tokens[idx].GetSuggestionFromDB(db, &wg)
	}
	wg.Wait()

	var sf = "%s"
	var suggestedQuery bytes.Buffer
	for idx, t := range tokens {
		sf = "%s"
		if format && t.SuggestionScore > 0.0 {
			sf = suggestionFormat
		}
		suggestedQuery.WriteString(fmt.Sprintf(sf, t.Suggestion))
		if idx < len(tokens)-1 {
			suggestedQuery.WriteString(" ")
		}
	}
	return suggestedQuery.String(), nil
}
