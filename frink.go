package frink

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"

	// import postgres driver
	_ "github.com/lib/pq"
)

var specialChars = "`~!@#$%^&*()-_+=|\\{}[]:;'\"/?.,><"
var dbQuery = "SELECT name, similarity(name, '%s') AS sml FROM countries WHERE name ILIKE '%s' ORDER BY sml DESC, name LIMIT 5"
var suggestionFormat = "<i>%s</i>"

// Frink type is a core type to bind methods to
type Frink struct{}

type suggestion struct {
	value        string
	score        float32
	editDistance float32
}

type token struct {
	Original       string
	Suggestions    []suggestion
	Order          int
	HasSuggestions bool
}

func (t *token) copyOriginalToSuggestion() {
	s := suggestion{value: t.Original, score: 0.0, editDistance: 0}
	t.Suggestions = append(t.Suggestions, s)
}

func (t *token) GetSuggestionFromDB(db *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()
	/*
		check Original is worth getting spell-checked, it's worth if
			- length >= 4
			- return if not
		query the db
		put in returned values into suggestions slice, put in a single original value in case of error anywhere
	*/
	if len(t.Original) < 3 {
		t.copyOriginalToSuggestion()
		return
	}

	tokenQuery := fmt.Sprintf(dbQuery, t.Original, "%"+t.Original+"%")
	//log.Println(tokenQuery)

	rows, err := db.Query(tokenQuery)
	if err != nil {
		log.Fatalln(err.Error())
		t.copyOriginalToSuggestion()
		return
	}
	defer rows.Close()

	for rows.Next() {
		// log.Println("--- found a row")
		var s suggestion
		err = rows.Scan(&s.value, &s.score)
		if err != nil {
			log.Fatalln(err.Error())
			s = suggestion{value: t.Original, score: 0.0, editDistance: 0}
		}
		// log.Println(s)
		t.Suggestions = append(t.Suggestions, s)
	}

	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		log.Fatalln(err.Error())
		t.copyOriginalToSuggestion()
		return
	}
	// mark HasSuggestions if we have received suggestions from the db so far
	t.HasSuggestions = len(t.Suggestions) > 0

	// if nobody assigned suggestions, copy original as default suggestion
	if len(t.Suggestions) == 0 {
		t.copyOriginalToSuggestion()
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
		t := token{Original: part, Order: idx + 1, HasSuggestions: false}
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
		if format && t.HasSuggestions {
			sf = suggestionFormat
		}
		suggestedQuery.WriteString(fmt.Sprintf(sf, t.Suggestions[0].value))
		if idx < len(tokens)-1 {
			suggestedQuery.WriteString(" ")
		}
	}
	return suggestedQuery.String(), nil
}
